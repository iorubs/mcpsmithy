package v1

import (
	"reflect"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
		want    *Config
	}{
		{
			name: "happy path",
			yaml: `version: "1"
project:
  name: test
  description: test project
  sources:
    local:
      docs:
        paths: ["*.md"]
    git:
      repo1:
        repo: https://github.com/org/repo
        paths: ["*.md"]
    scrape:
      site:
        urls: ["https://example.com"]
conventions:
  c1:
    description: d
tools:
  t1:
    description: d
    template: test
    options:
      maxResults: 10
    params:
      - name: q
        type: string
        default: hello
      - name: n
        type: number
        default: 42
      - name: v
        type: bool
        default: true
      - name: r
        type: number
        default: 3.14
      - name: count
        type: number
        constraints:
          min: 1
          max: 100
      - name: env
        type: string
        constraints:
          enum: [dev, staging, prod]
      - name: limit
        type: number
        constraints:
          enum: [1, 50, 100]
`,
			want: &Config{
				Version: "1",
				Project: Project{
					Name:        "test",
					Description: "test project",
					Sources: &ProjectSources{
						PullPolicy: PullPolicyIfNotPresent,
						Local: map[string]LocalSource{
							"docs": {
								Paths: []string{"*.md"},
								Index: new(true),
							},
						},
						Git: map[string]GitSource{
							"repo1": {
								Repo:  "https://github.com/org/repo",
								Paths: []string{"*.md"},
								Depth: 1,
								Index: new(true),
							},
						},
						Scrape: map[string]ScrapeSource{
							"site": {
								URLs:        []string{"https://example.com"},
								Index:       new(true),
								MaxPageSize: 2048,
								MaxPages:    20,
							},
						},
					},
				},
				Conventions: map[string]Convention{
					"c1": {Description: "d"},
				},
				Tools: map[string]Tool{
					"t1": {
						Description: "d",
						Template:    "test",
						Options:     map[string]any{"maxResults": 10},
						MaxOutputKB: 1024,
						Params: []ToolParam{
							{Name: "q", Type: ParamTypeString, Default: "hello"},
							{Name: "n", Type: ParamTypeNumber, Default: 42},
							{Name: "v", Type: ParamTypeBool, Default: true},
							{Name: "r", Type: ParamTypeNumber, Default: 3.14},
							{Name: "count", Type: ParamTypeNumber, Constraints: &ParamConstraints{
								Min: new(float64(1)), Max: new(float64(100)),
							}},
							{Name: "env", Type: ParamTypeString, Constraints: &ParamConstraints{
								Enum: []any{"dev", "staging", "prod"},
							}},
							{Name: "limit", Type: ParamTypeNumber, Constraints: &ParamConstraints{
								Enum: []any{1, 50, 100},
							}},
						},
					},
				},
			},
		},
		{
			name: "explicit values override defaults",
			yaml: `version: "1"
project:
  name: test
  description: test project
  sources:
    pullPolicy: always
    git:
      repo1:
        repo: https://github.com/org/repo
        paths: ["*.md"]
        depth: 5
    scrape:
      site:
        urls: ["https://example.com"]
        maxPageSize: 512
conventions:
  c1:
    description: d
tools:
  t1:
    description: d
    template: test
`,
			want: &Config{
				Version: "1",
				Project: Project{
					Name:        "test",
					Description: "test project",
					Sources: &ProjectSources{
						PullPolicy: PullPolicyAlways,
						Git: map[string]GitSource{
							"repo1": {
								Repo:  "https://github.com/org/repo",
								Paths: []string{"*.md"},
								Depth: 5,
								Index: new(true),
							},
						},
						Scrape: map[string]ScrapeSource{
							"site": {
								URLs:        []string{"https://example.com"},
								Index:       new(true),
								MaxPageSize: 512,
								MaxPages:    20,
							},
						},
					},
				},
				Conventions: map[string]Convention{
					"c1": {Description: "d"},
				},
				Tools: map[string]Tool{
					"t1": {
						Description: "d",
						Template:    "test",
						MaxOutputKB: 1024,
					},
				},
			},
		},
		{
			name:    "malformed YAML",
			yaml:    "version: \"1\"\nproject:\n  name: [bad",
			wantErr: "parsing config",
		},
		{
			name: "unknown field",
			yaml: `version: "1"
project:
  name: test
  bogusField: oops
`,
			wantErr: "parsing config",
		},
		{
			name: "template invalid syntax",
			yaml: `version: "1"
project:
  name: test
tools:
  bad:
    description: d
    template: "{{ if }}"
`,
			wantErr: "missing value for if",
		},
		{
			name: "template undeclared param ref",
			yaml: `version: "1"
project:
  name: test
tools:
  bad:
    description: d
    template: "Hello {{ .typoParam }}"
    params:
      - name: name
        type: string
`,
			wantErr: "typoParam",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Schema{}.Parse([]byte(tt.yaml))

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q does not contain %q", err, tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.want != nil && !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("Parse() mismatch\ngot:  %+v\nwant: %+v", cfg, tt.want)
			}
		})
	}
}

func TestSchemaRootType(t *testing.T) {
	s := Schema{}
	rt := s.RootType()
	if _, ok := rt.(Config); !ok {
		t.Errorf("expected Config, got %T", rt)
	}
}
