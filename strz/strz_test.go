package strz

import "testing"

func TestUnderscore(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{"blank", args{""}, ""},
		{"already_snake", args{"already_snake"}, "already_snake"},
		{"A", args{"A"}, "a"},
		{"HTTPRequest", args{"HTTPRequest"}, "http_request"},
		{"BatteryLifeValue", args{"BatteryLifeValue"}, "battery_life_value"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Underscore(tt.args.s); got != tt.want {
				t.Errorf("Underscore() = %v, want %v", got, tt.want)
			}
		})
	}
}
