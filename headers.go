package echoApi

import (
	"slices"
	"strconv"
	"strings"
)

type DefaultHeader struct {
	Announce       string `header:"x-auth-announce"`
	Channel        string `header:"x-auth-channel"`
	ChannelDetail  string `header:"x-auth-channel-detail"`
	Timestamp      string `header:"x-auth-timestamp"`
	Version        string `header:"x-auth-version"`
	Package        string `header:"x-auth-package"`
	Token          string `header:"x-auth-token"`
	Signature      string `header:"x-auth-signature"`
	AcceptLanguage string `header:"Accept-Language"`
}

// Accept-Language 解析
type LangQ struct {
	Lang string
	Q    float64
}

// ParseAcceptLanguage
// Accept-Language 解析
func ParseAcceptLanguage(acptLang string) []LangQ {
	var lqs []LangQ

	langQStrs := strings.Split(acptLang, ",")
	for _, langQStr := range langQStrs {
		trimedLangQStr := strings.Trim(langQStr, " ")

		langQ := strings.Split(trimedLangQStr, ";")
		if len(langQ) == 1 {
			lq := LangQ{langQ[0], 1}
			lqs = append(lqs, lq)
		} else {
			qp := strings.Split(langQ[1], "=")
			q, err := strconv.ParseFloat(qp[1], 64)
			if err != nil {
				panic(err)
			}
			lq := LangQ{langQ[0], q}
			lqs = append(lqs, lq)
		}
	}
	return lqs
}

// GetAcceptLanguage 获取语言  值返回排名第一的值
func GetAcceptLanguage(acptlang string) string {
	langs := ParseAcceptLanguage(acptlang)
	Language := "en"
	if len(langs) > 0 {
		slices.SortFunc(langs, func(a, b LangQ) int {
			if a.Q < b.Q {
				return 1
			}
			return -1
		})
		Language = langs[0].Lang
	}
	return Language
}

// ParseXForwardedFor ip链 解析
func ParseXForwardedFor(xForwardedFor string) []string {
	return strings.Split(xForwardedFor, ",")
}
