// Command converter 提供「Office 文档 → PDF」的独立转换服务。
//
// 之所以单独部署：LibreOffice 会带来数百 MB 依赖，把它从后端 API 镜像里拆出去，
// 既能让后端镜像保持轻量，也让文档转换可独立扩容与重启。
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	defaultPort       = "5010"
	defaultConcurrent = 2
	// 实测一份 468KB、含大量图片的 docx 转换耗时 87s，这里留足余量；
	// 需小于后端客户端的 converter_timeout_sec（300s），保证由服务先返回明确错误。
	defaultTimeoutSec = 240
)

func main() {
	port := envOr("PORT", defaultPort)
	concurrency := envInt("MAX_CONCURRENT", defaultConcurrent)
	if concurrency < 1 {
		concurrency = 1
	}
	timeoutSec := envInt("CONVERT_TIMEOUT_SEC", defaultTimeoutSec)
	if timeoutSec < 30 {
		timeoutSec = 30
	}
	sem := make(chan struct{}, concurrency)
	convertTimeout := time.Duration(timeoutSec) * time.Second

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("/convert", func(w http.ResponseWriter, r *http.Request) {
		handleConvert(w, r, sem, convertTimeout)
	})

	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}
	log.Printf("[converter] listening on :%s (max concurrency=%d, timeout=%s)", port, concurrency, convertTimeout)
	log.Fatal(srv.ListenAndServe())
}

func handleConvert(w http.ResponseWriter, r *http.Request, sem chan struct{}, convertTimeout time.Duration) {
	if r.Method != http.MethodPost {
		http.Error(w, "仅支持 POST multipart/form-data，字段名 file", http.StatusMethodNotAllowed)
		return
	}
	select {
	case sem <- struct{}{}:
		defer func() { <-sem }()
	default:
		http.Error(w, "转换服务繁忙，请稍后重试", http.StatusTooManyRequests)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), convertTimeout)
	defer cancel()

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		http.Error(w, "解析上传内容失败: "+err.Error(), http.StatusBadRequest)
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "缺少 file 字段", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	workDir, err := os.MkdirTemp("", "convert-")
	if err != nil {
		http.Error(w, "创建临时目录失败", http.StatusInternalServerError)
		return
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	name := sanitizeFilename(header.Filename)
	if name == "" {
		http.Error(w, "文件名非法", http.StatusBadRequest)
		return
	}
	srcPath := filepath.Join(workDir, name)
	dst, err := os.Create(srcPath)
	if err != nil {
		http.Error(w, "保存上传文件失败", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(dst, file); err != nil {
		_ = dst.Close()
		http.Error(w, "保存上传文件失败", http.StatusInternalServerError)
		return
	}
	if err := dst.Close(); err != nil {
		http.Error(w, "保存上传文件失败", http.StatusInternalServerError)
		return
	}

	started := time.Now()
	out, err := runSoffice(ctx, srcPath, workDir)
	if err != nil {
		log.Printf("[converter] 转换失败 file=%s err=%v output=%s", name, err, out)
		http.Error(w, "文档转换失败: "+err.Error(), http.StatusInternalServerError)
		return
	}
	pdfPath := filepath.Join(workDir, strings.TrimSuffix(name, filepath.Ext(name))+".pdf")
	fi, err := os.Stat(pdfPath)
	if err != nil || fi.Size() == 0 {
		log.Printf("[converter] 未生成有效 PDF file=%s", name)
		http.Error(w, "文档转换失败：未生成有效 PDF", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", filepath.Base(pdfPath)))
	http.ServeFile(w, r, pdfPath)
	log.Printf("[converter] 转换成功 file=%s size=%d cost=%s", name, fi.Size(), time.Since(started))
}

// runSoffice 调用 LibreOffice 无头模式转换；每次转换使用独立的用户配置目录，
// 避免并发调用互相抢占同一个 LibreOffice 实例。
//
// 注意：这里不用 exec.CommandContext，而是自己把子进程放进独立进程组。
// LibreOffice 会派生 soffice.bin 等子进程，直接 kill 父进程会留下孤儿（僵尸），
// 某些异常文档还会让转换卡死——必须按进程组整体终止，才能释放并发槽位。
func runSoffice(ctx context.Context, srcPath, workDir string) (string, error) {
	bin := envOr("SOFFICE_BIN", "soffice")
	if p, err := exec.LookPath(bin); err == nil {
		bin = p
	}
	profile := filepath.Join(workDir, "profile")
	cmd := exec.Command(bin,
		"--headless",
		"--norestore",
		"--nolockcheck",
		"--invisible",
		"-env:UserInstallation=file://"+profile,
		"--convert-to", "pdf",
		"--outdir", workDir,
		srcPath,
	)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	out := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = out

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("启动转换进程失败: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-ctx.Done():
		killProcessGroup(cmd.Process.Pid)
		<-done
		return out.String(), fmt.Errorf("转换超时，已终止转换进程")
	case err := <-done:
		if err != nil {
			return out.String(), fmt.Errorf("转换进程异常退出: %w", err)
		}
		return out.String(), nil
	}
}

// killProcessGroup 终止整个进程组（负号 pid），避免残留 soffice 子进程
func killProcessGroup(pid int) {
	if pid <= 0 {
		return
	}
	_ = syscall.Kill(-pid, syscall.SIGKILL)
	_ = syscall.Kill(pid, syscall.SIGKILL)
}

// sanitizeFilename 去掉目录分隔符与相对路径片段，避免路径穿越
func sanitizeFilename(name string) string {
	base := filepath.Base(strings.TrimSpace(name))
	base = strings.ReplaceAll(base, "/", "")
	base = strings.ReplaceAll(base, "\\", "")
	if base == "." || base == ".." {
		return ""
	}
	return base
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
