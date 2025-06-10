package common

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
)

// CryptoManager 加密管理器
type CryptoManager struct {
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
	AESKey     []byte
}

// NewCryptoManager 创建新的加密管理器
func NewCryptoManager() *CryptoManager {
	return &CryptoManager{}
}

// GenerateRSAKeyPair 生成RSA密钥对
func (cm *CryptoManager) GenerateRSAKeyPair(bits int) error {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return fmt.Errorf("生成RSA密钥对失败: %v", err)
	}
	
	cm.PrivateKey = privateKey
	cm.PublicKey = &privateKey.PublicKey
	return nil
}

// GenerateAESKey 生成AES密钥
func (cm *CryptoManager) GenerateAESKey() error {
	key := make([]byte, 32) // AES-256
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return fmt.Errorf("生成AES密钥失败: %v", err)
	}
	cm.AESKey = key
	return nil
}

// SetAESKey 设置AES密钥
func (cm *CryptoManager) SetAESKey(key []byte) {
	cm.AESKey = key
}

// GetPublicKeyPEM 获取公钥的PEM格式
func (cm *CryptoManager) GetPublicKeyPEM() (string, error) {
	if cm.PublicKey == nil {
		return "", fmt.Errorf("公钥未设置")
	}
	
	pubKeyBytes, err := x509.MarshalPKIXPublicKey(cm.PublicKey)
	if err != nil {
		return "", fmt.Errorf("序列化公钥失败: %v", err)
	}
	
	pubKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PUBLIC KEY",
		Bytes: pubKeyBytes,
	})
	
	return string(pubKeyPEM), nil
}

// SetPublicKeyFromPEM 从PEM格式设置公钥
func (cm *CryptoManager) SetPublicKeyFromPEM(pemData string) error {
	block, _ := pem.Decode([]byte(pemData))
	if block == nil {
		return fmt.Errorf("解析PEM数据失败")
	}
	
	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("解析公钥失败: %v", err)
	}
	
	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("不是RSA公钥")
	}
	
	cm.PublicKey = rsaPubKey
	return nil
}

// EncryptAESKeyWithRSA 使用RSA加密AES密钥
func (cm *CryptoManager) EncryptAESKeyWithRSA(aesKey []byte) ([]byte, error) {
	if cm.PublicKey == nil {
		return nil, fmt.Errorf("公钥未设置")
	}
	
	encryptedKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, cm.PublicKey, aesKey, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA加密AES密钥失败: %v", err)
	}
	
	return encryptedKey, nil
}

// DecryptAESKeyWithRSA 使用RSA私钥解密AES密钥
func (cm *CryptoManager) DecryptAESKeyWithRSA(encryptedKey []byte) ([]byte, error) {
	if cm.PrivateKey == nil {
		return nil, fmt.Errorf("私钥未设置")
	}
	
	decryptedKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, cm.PrivateKey, encryptedKey, nil)
	if err != nil {
		return nil, fmt.Errorf("RSA解密AES密钥失败: %v", err)
	}
	
	return decryptedKey, nil
}

// EncryptWithAES 使用AES加密数据
func (cm *CryptoManager) EncryptWithAES(plaintext []byte) ([]byte, error) {
	if cm.AESKey == nil {
		return nil, fmt.Errorf("AES密钥未设置")
	}
	
	block, err := aes.NewCipher(cm.AESKey)
	if err != nil {
		return nil, fmt.Errorf("创建AES cipher失败: %v", err)
	}
	
	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM模式失败: %v", err)
	}
	
	// 生成随机nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("生成nonce失败: %v", err)
	}
	
	// 加密
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// DecryptWithAES 使用AES解密数据
func (cm *CryptoManager) DecryptWithAES(ciphertext []byte) ([]byte, error) {
	if cm.AESKey == nil {
		return nil, fmt.Errorf("AES密钥未设置")
	}
	
	block, err := aes.NewCipher(cm.AESKey)
	if err != nil {
		return nil, fmt.Errorf("创建AES cipher失败: %v", err)
	}
	
	// 创建GCM模式
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("创建GCM模式失败: %v", err)
	}
	
	// 检查密文长度
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("密文长度不足")
	}
	
	// 分离nonce和密文
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	
	// 解密
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("AES解密失败: %v", err)
	}
	
	return plaintext, nil
}

// EncodeMessage 编码消息（base64）
func EncodeMessage(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeMessage 解码消息（base64）
func DecodeMessage(encoded string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(encoded)
} 