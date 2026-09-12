//go:build !notuya

package tuya

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
)

// aesECBEncrypt encrypts with AES-ECB, PKCS#7 padded to 16 bytes.
func aesECBEncrypt(key, plain []byte) ([]byte, error) {
	bs := aes.BlockSize
	padded := make([]byte, len(plain))
	padded = append(padded, plain...)
	pad := bs - len(padded)%bs
	for i := 0; i < pad; i++ {
		padded = append(padded, byte(pad))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(padded))
	for i := 0; i < len(padded); i += bs {
		block.Encrypt(out[i:], padded[i:i+bs])
	}
	return out, nil
}

// aesECBEncryptNoPad encrypts data whose length is already a multiple of 16.
func aesECBEncryptNoPad(key, plain []byte) ([]byte, error) {
	if len(plain)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("tuya: ECB no-pad input length %d not multiple of 16", len(plain))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(plain))
	for i := 0; i < len(plain); i += aes.BlockSize {
		block.Encrypt(out[i:], plain[i:i+aes.BlockSize])
	}
	return out, nil
}

// aesECBDecrypt decrypts AES-ECB and strips PKCS#7 padding.
func aesECBDecrypt(key, data []byte) ([]byte, error) {
	if len(data) == 0 || len(data)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("tuya: ECB input length %d not multiple of 16", len(data))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	out := make([]byte, len(data))
	for i := 0; i < len(data); i += aes.BlockSize {
		block.Decrypt(out[i:], data[i:i+aes.BlockSize])
	}
	pad := int(out[len(out)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(out) {
		return nil, fmt.Errorf("tuya: invalid PKCS#7 padding length %d", pad)
	}
	for _, b := range out[len(out)-pad:] {
		if int(b) != pad {
			return nil, fmt.Errorf("tuya: invalid PKCS#7 padding")
		}
	}
	return out[:len(out)-pad], nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

func hmacEqual(a, b []byte) bool { return hmac.Equal(a, b) }

func newGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// gcmEncrypt returns iv||ciphertext||tag style output: ciphertext with the
// 16-byte tag appended (iv is passed separately, as in the tuya frames).
func gcmEncrypt(aead cipher.AEAD, iv, plain, aad []byte) ([]byte, error) {
	if len(iv) != 12 {
		return nil, fmt.Errorf("tuya: GCM iv must be 12 bytes, got %d", len(iv))
	}
	return aead.Seal(nil, iv, plain, aad), nil
}

func gcmDecrypt(aead cipher.AEAD, iv, ct, aad []byte) ([]byte, error) {
	if len(iv) != 12 {
		return nil, fmt.Errorf("tuya: GCM iv must be 12 bytes, got %d", len(iv))
	}
	return aead.Open(nil, iv, ct, aad)
}

func randomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("tuya: cannot read random bytes: " + err.Error())
	}
	return b
}
