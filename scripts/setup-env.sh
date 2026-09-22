#!/usr/bin/env bash
set -euo pipefail

# scripts/setup-env.sh
# Prepares the local environment for building, running, and testing harnez.

os=$(uname -s)

log_pass() {
   printf '  ✓ %s\n' "$*"
}

log_fail() {
   printf 'ERROR: %s\n' "$*" >&2
   exit 1
}

check_or_install_minisign() {
   if command -v minisign >/dev/null 2>&1
   then log_pass "minisign is installed: $(command -v minisign)"
        return 0
   fi

   printf 'Installing minisign...\n'
   case "$os" in
   Darwin)
      if ! command -v brew >/dev/null 2>&1
      then log_fail "brew not found, cannot install minisign automatically"
      fi
      brew install minisign
      ;;
   Linux)
      if command -v apt-get >/dev/null 2>&1
      then export DEBIAN_FRONTEND=noninteractive
           if test "$(id -u)" -eq 0
           then apt-get update -y
                apt-get install -y minisign git git-lfs ca-certificates build-essential
           elif command -v sudo >/dev/null 2>&1
           then sudo apt-get update -y
                sudo apt-get install -y minisign git git-lfs ca-certificates build-essential
           else log_fail "apt-get requires root or sudo to install minisign"
           fi
      else log_fail "No supported package manager found to install minisign on Linux"
      fi
      ;;
   *)
      log_fail "Unsupported OS $os for automatic dependency installation"
      ;;
   esac

   if command -v minisign >/dev/null 2>&1
   then log_pass "minisign successfully installed"
   else log_fail "minisign installation failed"
   fi
}

check_go() {
   if ! command -v go >/dev/null 2>&1
   then log_fail "Go toolchain is not installed. Please install Go 1.24 or later."
   fi

   local go_version
   go_version=$(go version)
   log_pass "Go toolchain found: $go_version"
}

check_make() {
   if ! command -v make >/dev/null 2>&1
   then log_fail "make is not installed. Please install GNU Make."
   fi

   log_pass "Make found: $(command -v make)"
}

check_git() {
   if ! command -v git >/dev/null 2>&1
   then log_fail "git is not installed."
   fi

   log_pass "Git found: $(command -v git)"

   if command -v git-lfs >/dev/null 2>&1
   then git lfs install --system >/dev/null 2>&1 || git lfs install >/dev/null 2>&1 || true
        log_pass "Git LFS found and initialized"
   else printf '  ! git-lfs not found (optional, recommended for large assets)\n'
   fi
}

download_go_modules() {
   printf 'Downloading Go module dependencies...\n'
   go mod download
   log_pass "Go modules downloaded successfully"
}

printf '=== Setting up harnez development environment ===\n'
check_make
check_git
check_go
check_or_install_minisign
download_go_modules
printf '=== Environment setup complete ===\n'
