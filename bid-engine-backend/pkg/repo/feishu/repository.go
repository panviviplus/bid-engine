package feishu

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"bid-engine/lib/common/storage"
	"bid-engine/pkg/db/model"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrAlreadyBound = errors.New("飞书身份或当前账号已有关联")
var ErrNoAlternateLogin = errors.New("请先设置其他登录方式，再解除飞书关联")
var ErrUserUnavailable = errors.New("用户账号不可用")

type Repository struct{ db *gorm.DB }

func NewRepository() *Repository { return &Repository{db: storage.GetDB()} }

func (r *Repository) BindingByOpen(ctx context.Context, appID, openID string) (*model.FeishuAccountBinding, error) {
	var item model.FeishuAccountBinding
	err := r.db.WithContext(ctx).Where("app_id = ? AND open_id = ?", appID, openID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *Repository) BindingByUser(ctx context.Context, appID string, userID int64) (*model.FeishuAccountBinding, error) {
	var item model.FeishuAccountBinding
	err := r.db.WithContext(ctx).Where("app_id = ? AND user_id = ?", appID, userID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &item, err
}

func (r *Repository) LoginOrRegister(ctx context.Context, appID, openID, tenantKey string) (int64, error) {
	var userID int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var bound model.FeishuAccountBinding
		err := tx.Where("app_id = ? AND open_id = ?", appID, openID).First(&bound).Error
		if err == nil {
			userID = bound.UserID
			return availableUser(tx, userID)
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		hash := sha256.Sum256([]byte(appID + ":" + openID))
		username := "fs_" + hex.EncodeToString(hash[:20])
		now := time.Now().Unix()
		user := &model.User{
			Username: username, Nickname: "飞书用户" + username[len(username)-6:],
			CompanyID: 0, Mobile: "", Email: "", Password: "", Status: 1,
			Role: 0, CreateTime: now, UpdateTime: now,
		}
		if err := tx.Create(user).Error; err != nil {
			return err
		}
		binding := &model.FeishuAccountBinding{AppID: appID, OpenID: openID, TenantKey: tenantKey, UserID: user.UserID, NotifyEnabled: 1}
		if err := tx.Create(binding).Error; err != nil {
			return err
		}
		userID = user.UserID
		return nil
	})
	if err != nil {
		var sqlErr *mysqlDriver.MySQLError
		if errors.As(err, &sqlErr) && sqlErr.Number == 1062 {
			// A concurrent completion may have committed the binding first.
			bound, lookupErr := r.BindingByOpen(ctx, appID, openID)
			if lookupErr == nil && bound != nil {
				if e := availableUser(r.db.WithContext(ctx), bound.UserID); e == nil {
					return bound.UserID, nil
				}
			}
		}
		return 0, err
	}
	return userID, nil
}

func availableUser(db *gorm.DB, userID int64) error {
	var user model.User
	if err := db.Where("user_id = ?", userID).First(&user).Error; err != nil {
		return err
	}
	if user.Status != 1 {
		return ErrUserUnavailable
	}
	return nil
}

func (r *Repository) Bind(ctx context.Context, appID, openID, tenantKey string, userID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := availableUser(tx, userID); err != nil {
			return err
		}
		var existing model.FeishuAccountBinding
		err := tx.Where("app_id = ? AND (open_id = ? OR user_id = ?)", appID, openID, userID).First(&existing).Error
		if err == nil {
			if existing.OpenID == openID && existing.UserID == userID {
				return nil
			}
			return ErrAlreadyBound
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		item := &model.FeishuAccountBinding{AppID: appID, OpenID: openID, TenantKey: tenantKey, UserID: userID, NotifyEnabled: 1}
		if err := tx.Create(item).Error; err != nil {
			var sqlErr *mysqlDriver.MySQLError
			if errors.As(err, &sqlErr) && sqlErr.Number == 1062 {
				return ErrAlreadyBound
			}
			return err
		}
		return nil
	})
}

func (r *Repository) Unbind(ctx context.Context, appID string, userID int64) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var user model.User
		if err := tx.Where("user_id = ?", userID).First(&user).Error; err != nil {
			return err
		}
		if user.Password == "" {
			return ErrNoAlternateLogin
		}
		return tx.Where("app_id = ? AND user_id = ?", appID, userID).Delete(&model.FeishuAccountBinding{}).Error
	})
}

func (r *Repository) SetNotify(ctx context.Context, appID string, userID int64, enabled bool) error {
	value := 0
	if enabled {
		value = 1
	}
	res := r.db.WithContext(ctx).Model(&model.FeishuAccountBinding{}).
		Where("app_id = ? AND user_id = ?", appID, userID).Update("notify_enabled", value)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *Repository) ExtraProfile(ctx context.Context, userID int64) (*model.UserExtraProfile, error) {
	var item model.UserExtraProfile
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return &model.UserExtraProfile{UserID: userID}, nil
	}
	return &item, err
}

func (r *Repository) SaveExtraProfile(ctx context.Context, userID int64, mobile, company string) error {
	item := &model.UserExtraProfile{UserID: userID, ContactMobile: mobile, CompanyDisplayName: company}
	return r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"contact_mobile": mobile, "company_display_name": company, "updated_at": time.Now(),
		}),
	}).Create(item).Error
}

type Recipient struct {
	UserID        int64  `gorm:"column:user_id"`
	OpenID        string `gorm:"column:open_id"`
	NotifyEnabled int32  `gorm:"column:notify_enabled"`
	Status        int32  `gorm:"column:status"`
}

func (r *Repository) Recipients(ctx context.Context, appID string, userIDs []int64) (map[int64]Recipient, error) {
	out := make(map[int64]Recipient)
	if len(userIDs) == 0 {
		return out, nil
	}
	var rows []Recipient
	err := r.db.WithContext(ctx).Table("feishu_account_binding AS b").
		Select("b.user_id, b.open_id, b.notify_enabled, u.status").
		Joins("JOIN user AS u ON u.user_id = b.user_id").
		Where("b.app_id = ? AND b.user_id IN ?", appID, userIDs).Find(&rows).Error
	for _, row := range rows {
		out[row.UserID] = row
	}
	return out, err
}

func (r *Repository) ClaimDueAlerts(ctx context.Context, now time.Time, limit int) ([]model.TenderIntelAlert, error) {
	var items []model.TenderIntelAlert
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("(feishu_push_status = 'pending' OR feishu_push_status = 'processing') AND (feishu_push_next_at IS NULL OR feishu_push_next_at <= ?)", now).
			Order("id ASC").Limit(limit).Find(&items).Error; err != nil {
			return err
		}
		if len(items) == 0 {
			return nil
		}
		ids := make([]int64, 0, len(items))
		for _, item := range items {
			ids = append(ids, item.ID)
		}
		return tx.Model(&model.TenderIntelAlert{}).Where("id IN ?", ids).
			Updates(map[string]any{"feishu_push_status": "processing", "feishu_push_next_at": now.Add(2 * time.Minute)}).Error
	})
	return items, err
}

func (r *Repository) FinishAlert(ctx context.Context, id int64, status, messageID, errorCode string, next *time.Time) error {
	updates := map[string]any{"feishu_push_status": status, "feishu_push_attempts": gorm.Expr("feishu_push_attempts + 1"),
		"feishu_push_message_id": messageID, "feishu_push_last_error": errorCode, "feishu_push_next_at": next}
	if status == "sent" {
		updates["feishu_push_sent_at"] = time.Now()
	}
	res := r.db.WithContext(ctx).Model(&model.TenderIntelAlert{}).Where("id = ? AND feishu_push_status = 'processing'", id).Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected != 1 {
		return fmt.Errorf("reminder %d delivery lease lost", id)
	}
	return nil
}
