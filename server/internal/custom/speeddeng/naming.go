package speeddeng

import (
	"strings"
)

const speedDengSuffix = "-雷速蹬"

func speedDengDisplayName(value string) (string, string) {
	name := strings.TrimSpace(value)
	if strings.HasSuffix(name, speedDengSuffix) {
		name = strings.TrimSpace(strings.TrimSuffix(name, speedDengSuffix))
	}
	if index := strings.IndexAny(name, "(（"); index >= 0 {
		name = strings.TrimSpace(name[:index])
	}
	for _, suffix := range []string{"员工用", "自用"} {
		if strings.HasSuffix(name, suffix) && len(strings.TrimSuffix(name, suffix)) > 0 {
			name = strings.TrimSpace(strings.TrimSuffix(name, suffix))
			break
		}
	}
	if name == "" || strings.EqualFold(name, "none") || strings.EqualFold(name, "__unknown__") || name == "未识别" {
		return "__unknown__::speed_deng", "未识别" + speedDengSuffix
	}
	if strings.HasPrefix(name, "北海") {
		return "beihai::speed_deng", "北海" + speedDengSuffix
	}
	return strings.ToLower(name) + "::speed_deng", name + speedDengSuffix
}
