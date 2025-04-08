package dao

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"time"

	"github.com/ego-component/egorm"
)

const (
	KEYSIZE = 32
)

// Provider 供应商模型
type Provider struct {
	ID      int64  `gorm:"primaryKey;autoIncrement;comment:'供应商ID'"`
	Name    string `gorm:"type:VARCHAR(64);NOT NULL;uniqueIndex:idx_name_channel;comment:'供应商名称'"`
	Code    string `gorm:"type:VARCHAR(64);NOT NULL;comment:'供应商编码, tencentCloud, aliyun'"`
	Channel string `gorm:"type:ENUM('SMS','EMAIL','IN_APP');NOT NULL;uniqueIndex:idx_name_channel;comment:'支持的渠道'"`

	Endpoint  string `gorm:"type:VARCHAR(255);NOT NULL;comment:'API入口地址'"`
	RegionID  string
	APIKey    string `gorm:"type:VARCHAR(255);NOT NULL;comment:'API密钥，明文'"`
	APISecret string `gorm:"type:VARCHAR(512);NOT NULL;comment:'API密钥,加密'"`
	APPID     string `gorm:"type:VARCHAR(512);comment:'应用ID，仅腾讯云使用'"`

	Weight           int    `gorm:"type:INT;NOT NULL;comment:'权重'"`
	QPSLimit         int    `gorm:"type:INT;NOT NULL;comment:'每秒请求数限制'"`
	DailyLimit       int    `gorm:"type:INT;NOT NULL;comment:'每日请求数限制'"`
	AuditCallbackURL string `gorm:"type:VARCHAR(256);comment:'回调URL，供应商通知审核结果'"`
	Status           string `gorm:"type:ENUM('ACTIVE','INACTIVE');NOT NULL;DEFAULT:'ACTIVE';comment:'状态，启用-ACTIVE，禁用-INACTIVE'"`
	Ctime            int64
	Utime            int64
}

// TableName 重命名表
func (Provider) TableName() string {
	return "providers"
}

type ProviderDAO interface {
	// Create 创建供应商
	Create(ctx context.Context, provider Provider) (Provider, error)
	// Update 更新供应商
	Update(ctx context.Context, provider Provider) error
	// FindByID 根据ID查找供应商
	FindByID(ctx context.Context, id int64) (Provider, error)
	// FindByChannel 查找指定渠道的所有供应商
	FindByChannel(ctx context.Context, channel string) ([]Provider, error)
}

type providerDAO struct {
	db         *egorm.Component
	encryptKey []byte
}

func NewProviderDAO(db *egorm.Component, encryptKey string) ProviderDAO {
	// 确保加密密钥长度为32字节
	key := make([]byte, KEYSIZE)
	copy(key, encryptKey)
	return &providerDAO{
		db:         db,
		encryptKey: key,
	}
}

// encrypt 使用AES-GCM加密
func (p *providerDAO) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(p.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decrypt 使用AES-GCM解密
func (p *providerDAO) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(p.encryptKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return "", errors.New("ciphertext太短了")
	}

	nonce := ciphertext[:gcm.NonceSize()]
	ciphertext = ciphertext[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// Create 创建供应商
func (p *providerDAO) Create(ctx context.Context, provider Provider) (Provider, error) {
	now := time.Now().Unix()
	provider.Ctime = now
	provider.Utime = now

	apiSecret := provider.APISecret
	encryptedSecret, err := p.encrypt(apiSecret)
	if err != nil {
		return Provider{}, err
	}
	provider.APISecret = encryptedSecret

	if err := p.db.WithContext(ctx).Create(&provider).Error; err != nil {
		return Provider{}, err
	}

	provider.APISecret = apiSecret

	return provider, nil
}

// Update 更新供应商
func (p *providerDAO) Update(ctx context.Context, provider Provider) error {
	provider.Utime = time.Now().Unix()

	// 构建更新字段映射
	updates := map[string]interface{}{
		"name":               provider.Name,
		"code":               provider.Code,
		"channel":            provider.Channel,
		"endpoint":           provider.Endpoint,
		"api_key":            provider.APIKey,
		"weight":             provider.Weight,
		"qps_limit":          provider.QPSLimit,
		"daily_limit":        provider.DailyLimit,
		"audit_callback_url": provider.AuditCallbackURL,
		"status":             provider.Status,
		"utime":              provider.Utime,
	}

	if provider.APISecret != "" {
		encryptedSecret, err := p.encrypt(provider.APISecret)
		if err != nil {
			return err
		}
		updates["api_secret"] = encryptedSecret
	}

	// 直接更新，无需显式事务
	return p.db.WithContext(ctx).Model(&Provider{}).Where("id = ?", provider.ID).Updates(updates).Error
}

// FindByID 根据ID查找供应商
func (p *providerDAO) FindByID(ctx context.Context, id int64) (Provider, error) {
	var provider Provider
	err := p.db.WithContext(ctx).Where("id = ?", id).First(&provider).Error
	if err != nil {
		return Provider{}, err
	}

	if provider.APISecret != "" {
		decryptedSecret, err := p.decrypt(provider.APISecret)
		if err != nil {
			return Provider{}, err
		}
		provider.APISecret = decryptedSecret
	}

	return provider, nil
}

// FindByChannel 查找指定渠道的所有供应商
func (p *providerDAO) FindByChannel(ctx context.Context, channel string) ([]Provider, error) {
	var providers []Provider
	err := p.db.WithContext(ctx).Where("channel = ? AND status = ?", channel, "ACTIVE").Find(&providers).Error
	if err != nil {
		return nil, err
	}

	for i := range providers {
		if providers[i].APISecret == "" {
			continue
		}
		decryptedSecret, err := p.decrypt(providers[i].APISecret)
		if err != nil {
			return nil, err
		}
		providers[i].APISecret = decryptedSecret
	}

	return providers, nil
}
