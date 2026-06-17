package document

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// baht converts satang (int32) to a Thai-formatted baht string, e.g. "1,500.00".
func baht(satang int32) string {
	b := float64(satang) / 100.0
	whole := int64(math.Abs(b))
	frac := int(math.Round(math.Abs(b)*100)) % 100

	// insert thousands separator
	s := fmt.Sprintf("%d", whole)
	if len(s) > 3 {
		var parts []string
		for len(s) > 3 {
			parts = append([]string{s[len(s)-3:]}, parts...)
			s = s[:len(s)-3]
		}
		parts = append([]string{s}, parts...)
		s = strings.Join(parts, ",")
	}

	if b < 0 {
		return fmt.Sprintf("-%s.%02d", s, frac)
	}
	return fmt.Sprintf("%s.%02d", s, frac)
}

// thaiDate formats time to Thai Buddhist Era date string: "17 มิถุนายน 2569".
func thaiDate(t time.Time) string {
	months := []string{
		"", "มกราคม", "กุมภาพันธ์", "มีนาคม", "เมษายน",
		"พฤษภาคม", "มิถุนายน", "กรกฎาคม", "สิงหาคม",
		"กันยายน", "ตุลาคม", "พฤศจิกายน", "ธันวาคม",
	}
	be := t.Year() + 543
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()], be)
}

// thaiDateTime formats time to Thai BE date + time: "17 มิถุนายน 2569 เวลา 14:30 น."
func thaiDateTime(t time.Time) string {
	return fmt.Sprintf("%s เวลา %02d:%02d น.", thaiDate(t), t.Hour(), t.Minute())
}
