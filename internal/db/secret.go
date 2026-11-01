package db

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// 凭证：值只在 secret 表（AES-256-GCM 密文），库外只有 vault.key 一把主密钥
//（docs/凭证清单.md，禁令 #19）。value 列存 nonce||ciphertext。

// LoadOrCreateVaultKey 读 vault.key；不存在则生成 32B 随机钥（0600）。
func LoadOrCreateVaultKey(path string) ([]byte, error) {
	if k, err := os.ReadFile(path); err == nil {
		if len(k) != 32 {
			return nil, fmt.Errorf("vault.key 长度 %d≠32", len(k))
		}
		return k, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, k, 0o600); err != nil {
		return nil, err
	}
	return k, nil
}

// EncryptSecret AES-256-GCM 加密：输出 nonce||ciphertext。
func EncryptSecret(key, plaintext []byte) ([]byte, error) {
	g, err := gcm(key)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptSecret 解 nonce||ciphertext。
func DecryptSecret(key, blob []byte) ([]byte, error) {
	g, err := gcm(key)
	if err != nil {
		return nil, err
	}
	if len(blob) < g.NonceSize() {
		return nil, fmt.Errorf("密文过短")
	}
	return g.Open(nil, blob[:g.NonceSize()], blob[g.NonceSize():], nil)
}

func gcm(key []byte) (cipher.AEAD, error) {
	b, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(b)
}

// PutSecret 写一条凭证（value 已是密文）。
func (d *DB) PutSecret(ctx context.Context, name string, ciphertext []byte, roleACL []string) error {
	now := d.now().UnixNano()
	_, err := d.db.ExecContext(ctx,
		`INSERT INTO secret (name, value, role_acl, created_at, rotated_at) VALUES (?,?,?,?,?)
		 ON CONFLICT(name) DO UPDATE SET value=excluded.value, role_acl=excluded.role_acl, rotated_at=excluded.rotated_at`,
		cargs(name, ciphertext, strings.Join(roleACL, ","), now, now)...)
	return err
}

// SecretBlob 不过 ACL 直取密文——网关内部机械闸专用（板 token 对验）；
// 席位侧读径仍只有 GetSecret（role_acl 放行）。
func (d *DB) SecretBlob(ctx context.Context, name string) (ciphertext []byte, ok bool, err error) {
	var blob []byte
	err = d.db.QueryRowContext(ctx, `SELECT value FROM secret WHERE name=?`, name).Scan(&blob)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return blob, true, nil
}

// GetSecret 按 role_acl 放行取密文——调用方解密。acl 为空串＝不授任何席。
func (d *DB) GetSecret(ctx context.Context, name, role string) (ciphertext []byte, ok bool, err error) {
	var blob []byte
	var acl string
	err = d.db.QueryRowContext(ctx, `SELECT value, role_acl FROM secret WHERE name=?`, name).
		Scan(&blob, &acl)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	for _, r := range strings.Split(acl, ",") {
		if r == role {
			return blob, true, nil
		}
	}
	return nil, false, nil
}
