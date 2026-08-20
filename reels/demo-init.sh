HARNEZ_FIXTURE="$PWD/reels/demo-harnez-001.ansi"

rm -rf my-project >/dev/null

harnez() {
   if test "${1:-}" = "usage"
   then cat "${HARNEZ_FIXTURE}"
   else command harnez "$@"
   fi
}
