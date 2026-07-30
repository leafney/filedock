package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/leafney/filedock/internal/model"
	"github.com/leafney/filedock/pkg/errc"
	"github.com/leafney/filedock/pkg/errx"
	"golang.org/x/text/unicode/norm"
	"gorm.io/gorm"
)

var defaultNicknameCandidates = []string{
	"宋江", "卢俊义", "吴用", "公孙胜", "关胜", "林冲", "秦明", "呼延灼", "花荣", "柴进", "李应", "朱仝", "鲁智深", "武松", "董平", "张清", "杨志", "徐宁", "索超", "戴宗", "刘唐", "李逵", "史进", "穆弘", "雷横", "李俊", "阮小二", "张横", "阮小五", "张顺", "阮小七", "杨雄", "石秀", "解珍", "解宝", "燕青",
	"刘备", "关羽", "张飞", "诸葛亮", "赵云", "马超", "黄忠", "黄月英", "孙权", "周瑜", "鲁肃", "吕蒙", "陆逊", "曹操", "司马懿", "夏侯惇", "典韦", "许褚", "郭嘉", "荀彧", "张辽", "徐晃", "甘宁", "太史慈", "魏延", "姜维", "庞统", "法正", "华佗", "貂蝉",
}

type NicknameSvc struct {
	db         *gorm.DB
	candidates []string
}

func NewNicknameSvc(db *gorm.DB) (*NicknameSvc, error) {
	if db == nil {
		return nil, fmt.Errorf("nickname database is required")
	}
	return &NicknameSvc{db: db, candidates: append([]string(nil), defaultNicknameCandidates...)}, nil
}

func NormalizeNickname(value string) (displayName, key string, err error) {
	if !utf8.ValidString(value) {
		return "", "", errx.New(errc.ErrDisplayName, nil)
	}
	displayName = strings.TrimSpace(norm.NFKC.String(value))
	if displayName == "" {
		return "", "", errx.New(errc.ErrDisplayName, nil)
	}
	runes := []rune(displayName)
	if len(runes) < 2 || len(runes) > 20 {
		return "", "", errx.New(errc.ErrDisplayName, nil)
	}
	for _, r := range runes {
		if unicode.IsControl(r) || unicode.In(r, unicode.Cf) || unicode.Is(unicode.Zl, r) || unicode.Is(unicode.Zp, r) {
			return "", "", errx.New(errc.ErrDisplayName, nil)
		}
	}
	key = strings.ToLower(displayName)
	switch key {
	case "filedock", "system", "admin", "administrator", "root", "管理员", "系统管理员":
		return "", "", errx.New(errc.ErrDisplayName, nil)
	}
	return displayName, key, nil
}

func (s *NicknameSvc) Random() (string, error) {
	if s == nil || s.db == nil {
		return "", fmt.Errorf("nickname service is nil")
	}
	for attempt := 0; attempt < 100; attempt++ {
		candidate, err := s.randomCandidate()
		if err != nil {
			return "", err
		}
		if _, _, err := NormalizeNickname(candidate); err != nil {
			continue
		}
		var count int64
		if err := s.db.Model(&model.User{}).Where("display_name_key = ? AND status = ?", strings.ToLower(candidate), model.UserStatusActive).Count(&count).Error; err != nil {
			return "", fmt.Errorf("check nickname availability: %w", err)
		}
		if count == 0 {
			return candidate, nil
		}
	}
	return "", errx.New(errc.ErrNicknameTaken, nil)
}

func (s *NicknameSvc) randomCandidate() (string, error) {
	if len(s.candidates) == 0 {
		return "", fmt.Errorf("nickname candidates are empty")
	}
	max := big.NewInt(int64(len(s.candidates)))
	index, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("randomize nickname: %w", err)
	}
	return s.candidates[index.Int64()], nil
}

func (s *NicknameSvc) Reserve(tx *gorm.DB, displayName string) (string, string, error) {
	if tx == nil {
		return "", "", fmt.Errorf("nickname transaction is required")
	}
	displayName, key, err := NormalizeNickname(displayName)
	if err != nil {
		return "", "", err
	}
	var count int64
	if err := tx.Model(&model.User{}).Where("display_name_key = ? AND status = ?", key, model.UserStatusActive).Count(&count).Error; err != nil {
		return "", "", fmt.Errorf("check nickname uniqueness: %w", err)
	}
	if count > 0 {
		return "", "", errx.New(errc.ErrNicknameTaken, nil)
	}
	return displayName, key, nil
}
