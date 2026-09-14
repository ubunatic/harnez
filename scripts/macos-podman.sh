#!/usr/bin/env bash
set -euo pipefail

# macos-podman.sh — Run containerized macOS with KVM acceleration via Podman
# Image: docker.io/dockurr/macos:latest

container_name="macos-kvm"
macos_version="13"
http_port="8006"
vnc_port="5900"
cpu_cores="2"
ram_size="4G"
disk_size="64G"
storage_dir="./macos-storage"
auto_rm="false"
action="run"

usage() {
    cat <<'EOF'
Usage: scripts/macos-podman.sh [action] [flags]

Actions:
  run       Start macOS container (default)
  stop      Stop running macOS container
  status    Show status, ports, and KVM acceleration
  clean     Stop and remove container and storage directory

Flags:
  --rm             Remove container and storage on teardown (default: keep for resume)
  --version <ver>  macOS version to boot (default: 13; 11=BigSur, 12=Monterey, 13=Ventura, 14=Sonoma, 15=Sequoia)
  --name <name>    Container name (default: macos-kvm)
  --port <port>    Web noVNC HTTP port (default: 8006)
  --vnc-port <p>   VNC port (default: 5900)
  --cpu <cores>    CPU cores (default: 2)
  --ram <size>     RAM size (default: 4G)
  --disk <size>    Disk size (default: 64G)
  --storage <dir>  Storage path on host (default: ./macos-storage)
  -h, --help       Show this help message

Web UI (noVNC): http://localhost:8006
VNC:            localhost:5900
EOF
}

while test $# -gt 0
do case "$1" in
       run|stop|status|clean)
           action="$1"
           shift
           ;;
       --rm)
           auto_rm="true"
           shift
           ;;
       --version)
           macos_version="$2"
           shift 2
           ;;
       --name)
           container_name="$2"
           shift 2
           ;;
       --port)
           http_port="$2"
           shift 2
           ;;
       --vnc-port)
           vnc_port="$2"
           shift 2
           ;;
       --cpu)
           cpu_cores="$2"
           shift 2
           ;;
       --ram)
           ram_size="$2"
           shift 2
           ;;
       --disk)
           disk_size="$2"
           shift 2
           ;;
       --storage)
           storage_dir="$2"
           shift 2
           ;;
       -h|--help)
           usage
           exit 0
           ;;
       *)
           printf 'ERROR: unknown option %q\n\n' "$1" >&2
           usage >&2
           exit 1
           ;;
   esac
done

check_prerequisites() {
    if ! command -v podman >/dev/null 2>&1
    then printf 'ERROR: podman is not installed or not in PATH\n' >&2
         exit 1
    fi

    if ! test -e /dev/kvm
    then printf 'WARNING: /dev/kvm not found — macOS will run unaccelerated (very slow)\n' >&2
    fi
}

do_run() {
    check_prerequisites

    if podman container exists "$container_name" 2>/dev/null
    then
        if test "$(podman inspect -f '{{.State.Running}}' "$container_name" 2>/dev/null)" = "true"
        then printf 'Container %q is already running.\n' "$container_name"
             printf 'Web UI: http://localhost:%s\n' "$http_port"
             printf 'VNC:    localhost:%s\n' "$vnc_port"
             exit 0
        else printf 'Starting existing stopped container %q...\n' "$container_name"
             podman start "$container_name"
             printf 'Web UI: http://localhost:%s\n' "$http_port"
             printf 'VNC:    localhost:%s\n' "$vnc_port"
             exit 0
        fi
    fi

    mkdir -p "$storage_dir"

    run_args=(
        "run"
        "-d"
        "--name" "$container_name"
        "-p" "${http_port}:8006"
        "-p" "${vnc_port}:5900"
        "-e" "VERSION=${macos_version}"
        "-e" "CPU_CORES=${cpu_cores}"
        "-e" "RAM_SIZE=${ram_size}"
        "-e" "DISK_SIZE=${disk_size}"
        "-v" "${storage_dir}:/storage:Z"
        "--stop-timeout" "120"
    )

    if test -e /dev/kvm
    then run_args+=("--device" "/dev/kvm")
    fi

    if test -e /dev/net/tun
    then run_args+=("--device" "/dev/net/tun" "--cap-add" "NET_ADMIN")
    fi

    if test "$auto_rm" = "true"
    then run_args+=("--rm")
    fi

    run_args+=("docker.io/dockurr/macos:latest")

    printf 'Launching macOS (version %s) container via Podman...\n' "$macos_version"
    podman "${run_args[@]}"

    printf '\nContainer %q started successfully.\n' "$container_name"
    printf '  Web UI: http://localhost:%s\n' "$http_port"
    printf '  VNC:    localhost:%s\n' "$vnc_port"
    if test "$auto_rm" = "true"
    then printf '  Auto-remove: enabled (--rm)\n'
    else printf '  Storage: %s (preserved across restarts)\n' "$storage_dir"
    fi
}

do_stop() {
    if podman container exists "$container_name" 2>/dev/null
    then printf 'Stopping container %q...\n' "$container_name"
         podman stop "$container_name"
         printf 'Container stopped.\n'
    else printf 'Container %q does not exist or is already stopped.\n' "$container_name"
    fi
}

do_status() {
    if podman container exists "$container_name" 2>/dev/null
    then
        running=$(podman inspect -f '{{.State.Running}}' "$container_name" 2>/dev/null || echo "false")
        printf 'Container: %s (Running: %s)\n' "$container_name"
        if test "$running" = "true"
        then printf '  Web UI: http://localhost:%s\n' "$http_port"
             printf '  VNC:    localhost:%s\n' "$vnc_port"
             podman logs --tail 10 "$container_name"
        fi
    else printf 'Container %q is not present.\n' "$container_name"
    fi
}

do_clean() {
    if podman container exists "$container_name" 2>/dev/null
    then printf 'Removing container %q...\n' "$container_name"
         podman rm -f "$container_name" 2>/dev/null || true
    fi

    if test -d "$storage_dir"
    then printf 'Removing storage directory %s...\n' "$storage_dir"
         rm -rf "$storage_dir"
    fi
    printf 'Cleanup complete.\n'
}

case "$action" in
    run)
        do_run
        ;;
    stop)
        do_stop
        ;;
    status)
        do_status
        ;;
    clean)
        do_clean
        ;;
esac
