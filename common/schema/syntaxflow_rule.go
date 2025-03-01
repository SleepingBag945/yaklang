package schema

import (
	"database/sql/driver"
	"encoding/json"
	"github.com/yaklang/yaklang/common/utils"
	"github.com/yaklang/yaklang/common/yakgrpc/ypb"
	"strings"

	"github.com/jinzhu/gorm"
	"github.com/yaklang/yaklang/common/utils/yakunquote"
	"github.com/yaklang/yaklang/common/yak/yaklib/codec"
)

type SyntaxFlowRulePurposeType string
type SyntaxFlowRuleType string
type SyntaxFlowSeverity string

const (
	SFR_PURPOSE_AUDIT    SyntaxFlowRulePurposeType = "audit"
	SFR_PURPOSE_VULN     SyntaxFlowRulePurposeType = "vuln"
	SFR_PURPOSE_CONFIG   SyntaxFlowRulePurposeType = "config"
	SFR_PURPOSE_SECURITY SyntaxFlowRulePurposeType = "security"
)

const (
	SFR_SEVERITY_LOW      = "info"
	SFR_SEVERITY_WARNING  = "middle"
	SFR_SEVERITY_CRITICAL = "critical"
	SFR_SEVERITY_HIGH     = "high"
)

const (
	SFR_RULE_TYPE_YAK SyntaxFlowRuleType = "yak"
	SFR_RULE_TYPE_SF  SyntaxFlowRuleType = "sf"
)

func ValidRuleType(i any) SyntaxFlowRuleType {
	switch strings.ToLower(codec.AnyToString(i)) {
	case "yak", "y", "yaklang":
		return SFR_RULE_TYPE_YAK
	case "sf", "syntaxflow":
		return SFR_RULE_TYPE_SF
	default:
		return SFR_RULE_TYPE_SF
	}
}

func ValidSeverityType(i any) SyntaxFlowSeverity {
	switch strings.ToLower(yakunquote.TryUnquote(codec.AnyToString(i))) {
	case "info", "i", "low", "verbose", "debug", "prompt":
		return SFR_SEVERITY_LOW
	case "warning", "w", "middle", "mid", "warn":
		return SFR_SEVERITY_WARNING
	case "critical", "c", "fatal", "e", "essential":
		return SFR_SEVERITY_CRITICAL
	case "high", "h", "error":
		return SFR_SEVERITY_HIGH
	default:
		return SFR_SEVERITY_LOW
	}
}

func ValidPurpose(i any) SyntaxFlowRulePurposeType {
	result := yakunquote.TryUnquote(codec.AnyToString(i))
	switch strings.ToLower(result) {
	case "audit", "a", "audition":
		return SFR_PURPOSE_AUDIT
	case "vuln", "v", "vulnerability", "vul", "vulnerabilities", "weak", "weakness":
		return SFR_PURPOSE_VULN
	case "config", "c", "configuration", "conf", "configure":
		return SFR_PURPOSE_CONFIG
	case "security", "s", "secure", "securely", "secureity":
		return SFR_PURPOSE_SECURITY
	default:
		return SFR_PURPOSE_AUDIT
	}
}

type MapEx[K comparable, V any] map[K]V

func (m *MapEx[K, V]) Scan(value interface{}) error {
	return json.Unmarshal(codec.AnyToBytes(value), m)
}
func (m MapEx[K, V]) Value() (driver.Value, error) {
	return json.Marshal(m)
}

type SlicesEx[K comparable] []K

func (s *SlicesEx[K]) Scan(value interface{}) error {
	return json.Unmarshal(codec.AnyToBytes(value), s)
}

func (s *SlicesEx[K]) Value() (driver.Value, error) {
	return json.Marshal(s)
}

type ExtraDescInfo struct {
	Level     SyntaxFlowSeverity
	Purpose   SyntaxFlowRulePurposeType
	Msg       string
	OnlyMsg   bool
	ExtraInfo map[string]string
}

type SyntaxFlowRule struct {
	gorm.Model

	IsBuildInRule bool

	// Language is the language of the rule.
	// if the rule is not set, all languages will be used.
	Language string

	RuleName    string `gorm:"unique_index"`
	Title       string
	TitleZh     string
	Description string
	Tag         string
	AlertDesc   MapEx[string, *ExtraDescInfo] `gorm:"type:text"`
	// yak or sf
	Type     SyntaxFlowRuleType
	Severity SyntaxFlowSeverity
	Content  string

	// Purpose is the purpose of the rule.
	// audit / vuln / config / security / information
	Purpose SyntaxFlowRulePurposeType

	// DemoFileSystem will description the file system of the rule.
	// This is a json string.
	//    save map[string]quotedString
	TypicalHitFileSystem []byte
	Verified             bool

	// AllowIncluded is the rule can be included by other rules.
	// If the rule is included by other rules, the rule will not be shown in the result.
	AllowIncluded bool
	IncludedName  string
	OpCodes       string

	Hash string `json:"hash" gorm:"unique_index"`
}

func (s *SyntaxFlowRule) CalcHash() string {
	s.Hash = utils.CalcSha256(s.RuleName, s.Content, s.Tag)
	return s.Hash
}

func (s *SyntaxFlowRule) BeforeSave() error {
	s.CalcHash()
	s.Purpose = ValidPurpose(s.Purpose)
	s.Type = ValidRuleType(s.Type)
	s.Severity = ValidSeverityType(s.Severity)
	return nil
}
func (s *SyntaxFlowRule) GetAlertInfo(msg string) (string, bool) {
	if info, ok := s.AlertDesc[msg]; ok {
		if info.OnlyMsg {
			return info.Msg, true
		}
		return codec.AnyToString(info), true
	}
	return "", false
}

func (s *SyntaxFlowRule) ToGRPCModel() *ypb.SyntaxFlowRule {
	sfRule := &ypb.SyntaxFlowRule{
		IsBuildInRule: s.IsBuildInRule,
		Language:      s.Language,
		RuleName:      s.RuleName,
		Title:         s.Title,
		TitleZh:       s.TitleZh,
		Description:   s.Description,
		Type:          string(s.Type),
		Severity:      string(s.Severity),
		Content:       s.Content,
		Purpose:       string(s.Purpose),
		Verified:      s.Verified,
		AllowIncluded: s.AllowIncluded,
		IncludedName:  s.IncludedName,
		Hash:          s.Hash,
		Tag:           s.Tag,
	}
	return sfRule
}
