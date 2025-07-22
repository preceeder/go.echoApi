package echoApi

import (
	"log/slog"
	"math/rand"
	"time"
)

// 随机字符串
var letters = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func RandStr(str_len int) string {
	rand_bytes := make([]rune, str_len)
	for i := range rand_bytes {
		rand_bytes[i] = letters[rand.Intn(len(letters))]
	}
	return string(rand_bytes)
}

type Date time.Time

func (ts Date) MarshalJSON() ([]byte, error) {
	origin := time.Time(ts)
	return []byte(origin.Format(time.DateOnly)), nil
}

func (ts *Date) ToTime() time.Time {
	return time.Time(*ts)
}

// UnmarshalJSON implements the json.Unmarshaler interface.
// The time is expected to be a quoted string in RFC 3339 format.
func (ts *Date) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, string(data))
	if err != nil {
		slog.Error("Data UnmarshalJSON ", "error", err.Error())
		return err
	}
	*ts = Date(t)
	return nil
}

func (ts Date) ToString() string {
	return ts.ToTime().Format(time.DateOnly)
}
