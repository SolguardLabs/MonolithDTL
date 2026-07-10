package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const (
	BpsDenominator = 10_000
	MaxAmount      = Amount(9_000_000_000_000_000_000)
)

func A(v int64) Amount {
	return Amount(v)
}

func Add(a Amount, b Amount) Amount {
	if b > 0 && a > MaxAmount-b {
		return MaxAmount
	}
	if b < 0 && a < -MaxAmount-b {
		return -MaxAmount
	}
	return a + b
}

func Sub(a Amount, b Amount) Amount {
	return Add(a, -b)
}

func MinAmount(a Amount, b Amount) Amount {
	if a < b {
		return a
	}
	return b
}

func MaxAmountOf(a Amount, b Amount) Amount {
	if a > b {
		return a
	}
	return b
}

func Positive(a Amount) Amount {
	if a < 0 {
		return 0
	}
	return a
}

func ClampAmount(v Amount, lo Amount, hi Amount) Amount {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func MulBps(amount Amount, bps int) Amount {
	if amount == 0 || bps == 0 {
		return 0
	}
	return Amount((int64(amount) * int64(bps)) / BpsDenominator)
}

func AddBps(amount Amount, bps int) Amount {
	return amount + MulBps(amount, bps)
}

func SubBps(amount Amount, bps int) Amount {
	return amount - MulBps(amount, bps)
}

func RatioBps(numerator Amount, denominator Amount) int {
	if denominator <= 0 {
		return 0
	}
	return int((int64(numerator) * BpsDenominator) / int64(denominator))
}

func ShareOf(total Amount, shares Amount, supply Amount) Amount {
	if total <= 0 || shares <= 0 || supply <= 0 {
		return 0
	}
	return Amount((int64(total) * int64(shares)) / int64(supply))
}

func CeilDivAmount(a Amount, b Amount) Amount {
	if b <= 0 {
		return 0
	}
	if a <= 0 {
		return 0
	}
	return Amount((int64(a) + int64(b) - 1) / int64(b))
}

func FormatAmount(asset Asset, amount Amount) string {
	sign := ""
	value := int64(amount)
	if value < 0 {
		sign = "-"
		value = -value
	}
	if asset.Decimals <= 0 {
		return fmt.Sprintf("%s%d %s", sign, value, asset.Symbol)
	}
	scale := int64(1)
	for i := 0; i < asset.Decimals; i++ {
		scale *= 10
	}
	whole := value / scale
	fraction := value % scale
	return fmt.Sprintf("%s%d.%0*d %s", sign, whole, asset.Decimals, fraction, asset.Symbol)
}

func ParseUnits(text string, decimals int) (Amount, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0, fmt.Errorf("empty amount")
	}
	sign := int64(1)
	if strings.HasPrefix(trimmed, "-") {
		sign = -1
		trimmed = strings.TrimPrefix(trimmed, "-")
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("invalid decimal amount %q", text)
	}
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return 0, err
	}
	scale := int64(1)
	for i := 0; i < decimals; i++ {
		scale *= 10
	}
	fraction := int64(0)
	if len(parts) == 2 {
		raw := parts[1]
		if len(raw) > decimals {
			raw = raw[:decimals]
		}
		for len(raw) < decimals {
			raw += "0"
		}
		if raw != "" {
			fraction, err = strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return 0, err
			}
		}
	}
	return Amount(sign * (whole*scale + fraction)), nil
}

func StableID(prefix string, parts ...string) string {
	joined := prefix + ":" + strings.Join(parts, ":")
	sum := sha256.Sum256([]byte(joined))
	return prefix + "-" + hex.EncodeToString(sum[:])[:12]
}

func Digest(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])[:32]
}

func DebtKey(account string, asset string) string {
	return account + "|" + asset
}

func LocalDebtKey(product ProductKind, productID string, account string, asset string) string {
	return string(product) + "|" + productID + "|" + account + "|" + asset
}

func ProductKey(product ProductKind, productID string) string {
	return string(product) + "|" + productID
}

func StringIn(value string, options []string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}
	return false
}

func ProductAllowed(product ProductKind, allowed []ProductKind) bool {
	for _, item := range allowed {
		if item == product {
			return true
		}
	}
	return false
}

func CopyStringMap(in map[string]Amount) map[string]Amount {
	out := make(map[string]Amount, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func AmountToString(v Amount) string {
	return strconv.FormatInt(int64(v), 10)
}

func BoolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func NonEmpty(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func JoinLabels(labels ...string) string {
	clean := make([]string, 0, len(labels))
	for _, label := range labels {
		label = strings.TrimSpace(label)
		if label != "" {
			clean = append(clean, label)
		}
	}
	return strings.Join(clean, "/")
}

func AbsAmount(v Amount) Amount {
	if v < 0 {
		return -v
	}
	return v
}

func WeightedAmount(amount Amount, weightBps int, haircutBps int) Amount {
	weighted := MulBps(amount, weightBps)
	return SubBps(weighted, haircutBps)
}

func IsDust(amount Amount, threshold Amount) bool {
	return AbsAmount(amount) <= threshold
}

func MustPositive(label string, amount Amount) error {
	if amount <= 0 {
		return fmt.Errorf("%s must be positive", label)
	}
	return nil
}

func MustKnownAsset(assets map[string]Asset, asset string) error {
	if _, ok := assets[asset]; !ok {
		return fmt.Errorf("unknown asset %s", asset)
	}
	return nil
}

func CopyStrings(in []string) []string {
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func CopyProducts(in []ProductKind) []ProductKind {
	out := make([]ProductKind, len(in))
	copy(out, in)
	return out
}

