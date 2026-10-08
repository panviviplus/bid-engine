package utils

import (
	"fmt"
	"os"
	"strconv"
)

func ProcessTxtFileToMapSlice(absoluteFilePath string) ([]map[string]string, error) {
	fmt.Printf("Reading file: %s\n", absoluteFilePath)

	fileData, err := os.ReadFile(absoluteFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	content := string(fileData)
	runes := []rune(content)
	result := make([]map[string]string, 0, len(runes))

	for index, r := range runes {
		charMap := map[string]string{
			"text":  string(r),
			"index": strconv.Itoa(index + 1),
		}
		result = append(result, charMap)
	}
	fmt.Printf("Processed %d characters.\n", len(result))
	return result, nil
}
