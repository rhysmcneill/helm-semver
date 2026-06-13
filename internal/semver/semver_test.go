package semver

import (
	"testing"
)

func TestAnalyze(t *testing.T) {
	tests := []struct {
		name    string
		commits []string
		want    BumpType
	}{
		{
			name:    "empty commits",
			commits: []string{},
			want:    BumpNone,
		},
		{
			name:    "non-releasable commits only",
			commits: []string{"chore: update readme", "docs: fix typo", "ci: add workflow"},
			want:    BumpNone,
		},
		{
			name:    "single fix",
			commits: []string{"fix: correct loki retention default"},
			want:    BumpPatch,
		},
		{
			name:    "single feat",
			commits: []string{"feat: add chartmuseum support"},
			want:    BumpMinor,
		},
		{
			name:    "breaking change via !",
			commits: []string{"feat!: remove --registry-url flag"},
			want:    BumpMajor,
		},
		{
			name:    "breaking change via BREAKING CHANGE in body",
			commits: []string{"feat: new flag syntax", "BREAKING CHANGE: --registry-url renamed to --registry"},
			want:    BumpMajor,
		},
		{
			name:    "feat wins over fix",
			commits: []string{"fix: typo", "feat: add dry-run flag"},
			want:    BumpMinor,
		},
		{
			name:    "major wins over feat",
			commits: []string{"feat: add support", "feat!: breaking api change"},
			want:    BumpMajor,
		},
		{
			name:    "scoped fix",
			commits: []string{"fix(observability): correct service port"},
			want:    BumpPatch,
		},
		{
			name:    "scoped feat",
			commits: []string{"feat(registry): add ECR support"},
			want:    BumpMinor,
		},
		{
			name:    "scoped breaking",
			commits: []string{"refactor(git)!: change tag format"},
			want:    BumpMajor,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(tt.commits)
			if got != tt.want {
				t.Errorf("Analyze(%v) = %v, want %v", tt.commits, got, tt.want)
			}
		})
	}
}

func TestNext(t *testing.T) {
	tests := []struct {
		name    string
		current string
		bump    BumpType
		want    string
		wantErr bool
	}{
		{name: "patch bump", current: "0.1.0", bump: BumpPatch, want: "0.1.1"},
		{name: "minor bump", current: "0.1.0", bump: BumpMinor, want: "0.2.0"},
		{name: "major bump", current: "0.1.0", bump: BumpMajor, want: "1.0.0"},
		{name: "minor resets patch", current: "1.2.3", bump: BumpMinor, want: "1.3.0"},
		{name: "major resets minor and patch", current: "1.2.3", bump: BumpMajor, want: "2.0.0"},
		{name: "none returns same version", current: "0.5.0", bump: BumpNone, want: "0.5.0"},
		{name: "invalid version", current: "not-a-version", bump: BumpPatch, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Next(tt.current, tt.bump)
			if (err != nil) != tt.wantErr {
				t.Errorf("Next(%q, %v) error = %v, wantErr %v", tt.current, tt.bump, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("Next(%q, %v) = %q, want %q", tt.current, tt.bump, got, tt.want)
			}
		})
	}
}

func TestBumpTypeString(t *testing.T) {
	tests := []struct {
		bump BumpType
		want string
	}{
		{BumpNone, "none"},
		{BumpPatch, "patch"},
		{BumpMinor, "minor"},
		{BumpMajor, "major"},
	}
	for _, tt := range tests {
		if got := tt.bump.String(); got != tt.want {
			t.Errorf("BumpType(%d).String() = %q, want %q", tt.bump, got, tt.want)
		}
	}
}
