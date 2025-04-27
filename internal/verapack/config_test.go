package verapack

import (
	_ "embed"
	"testing"
)

type args struct {
	configBytes []byte
}

type testConfig struct {
	want           Config
	name           string
	args           args
	wantErr        bool
	validationFunc func(t *testing.T, tc testConfig, got Config)
}

func TestSetDefaults(t *testing.T) {
	tests := []testConfig{
		{
			// default bool value = [true|omitted|false] + target bool value = [false], should = target bool value = false
			// default bool value = [true|omitted|false] + target bool value = [true], should = target bool value = true
			// default bool value = [true] + target bool value = [omitted], should = target bool value = [true]
			name: "config bool value overrides vs. defaults bug",
			args: args{configBytes: []byte(`
default:
  create_profile: true
  auto_cleanup: false
  trust: true
applications:
  - app_name: Test
    create_profile: false
    auto_cleanup: true`)},
			wantErr: false,
			validationFunc: func(t *testing.T, tc testConfig, got Config) {
				// create_profile being used to check default true target false, expect false
				// auto_cleanup being used to check default false target true, expect true
				// trust being used to check default true target omitted, expect true
				if *got.Applications[0].CreateProfile || !*got.Applications[0].AutoCleanup || !*got.Applications[0].Trust {
					t.Errorf("SetDefaults() = create_profile=%t auto_cleanup=%t trust=%t, want create_profile=%t auto_cleanup=%t trust=%t", *got.Applications[0].CreateProfile, *got.Applications[0].AutoCleanup, *got.Applications[0].Trust, *got.Applications[0].CreateProfile, *got.Applications[0].AutoCleanup, *got.Applications[0].Trust)
				}
			},
		},
		{
			name: "config override default non-empty value",
			args: args{configBytes: []byte(`
default:
  type: directory
applications:
  - app_name: Test
    type: repo`)},
			want: Config{
				Applications: []Options{{
					Type: "repo",
				}},
			},
			wantErr: false,
			validationFunc: func(t *testing.T, tc testConfig, got Config) {
				if got.Applications[0].Type != tc.want.Applications[0].Type {
					t.Errorf("SetDefaults() = type=%s, want type=%s", got.Applications[0].Type, tc.want.Applications[0].Type)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SetDefaults(tt.args.configBytes)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetDefaults() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			tt.validationFunc(t, tt, got)
		})
	}
}
