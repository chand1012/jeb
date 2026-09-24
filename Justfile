DATE := `date +"%Y-%m-%d_%H:%M:%S"`
GIT_COMMIT := `git rev-parse HEAD`
VERSION_TAG := `git describe --tags --abbrev=0 2>/dev/null || echo "dev"`
LD_FLAGS := "-X github.com/chand1012/jeb/pkg/version.Version=" + VERSION_TAG + " -X github.com/chand1012/jeb/pkg/version.CommitHash=" + GIT_COMMIT + " -X github.com/chand1012/jeb/pkg/version.BuildDate=" + DATE

default:
    just --list --unsorted

tidy:
    go mod tidy

build:
    mkdir -p bin
    go build -ldflags "{{LD_FLAGS}}" -v -o bin/jeb

add command:
    cobra-cli add {{command}}

serve:
    go run main.go serve

clean:
    rm -rf bin
