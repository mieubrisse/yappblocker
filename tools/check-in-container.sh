#!/usr/bin/env bash
# Runs the check, the same `go -C tools run ./check` CI runs, inside a container that exists for this
# one run, so nothing the check starts can outlive it (Linear KEV-138). When the container's first
# process exits, the kernel kills everything left in its PID namespace, including a test helper that
# called setsid to escape a process-group kill and that `go test -timeout` would orphan. The network
# namespace takes every bound port with it, and the tmpfs takes every scratch file.
#
# The flags are the policy, not Docker: without them a container has no ceilings at all. Each is
# explained where it is set below.
#
# Needs Docker on a Linux host and nothing else: no Go, no C compiler. On the factory box, Docker
# comes from the dotfiles repo's roles/docker, whose header is the box's provisioning notes. Run
# from anywhere:
#   bash tools/check-in-container.sh
set -euo pipefail
script_dirpath="$(cd "$(dirname "${0}")" && pwd)"
repo_root_dirpath="$(cd "${script_dirpath}/.." && pwd)"

# The one check command, byte-identical to CI's, the pre-commit hook's and the docs'
CHECK_COMMAND=(go -C tools run ./check)

# Ceilings, derived from the full check measured on the factory box with no ceilings but these four
# CPUs (2026-11-22, recorded on Linear KEV-138): memory.peak 1.85 GB, /tmp 877 MB, at most 86 pids
# sampled every 0.2s, 78s wall clock. The memory peak comes from building the pinned check tools,
# not from the -race tests, and it includes the tmpfs, whose pages are charged to the container.
# Re-measure when the suite or a pin changes much.
MEMORY_LIMIT="4g"       # about 2x the measured peak
TMPFS_SIZE="2g"         # about 2x the measured /tmp
CPU_LIMIT="4"           # what was measured with; Go sizes GOMAXPROCS to it
PIDS_LIMIT="256"        # about 3x the sampled maximum, since sampling misses short-lived compilers
TIME_LIMIT_SECONDS="300" # about 4x on a loaded box, and under go test's own 10-minute -timeout so this ceiling ends a hang

# The warm-up has network and runs as root inside its container, so it gets ceilings of its own: the
# check's, and a time limit generous enough for a cold module cache on a slow link
WARM_TIME_LIMIT_SECONDS="600"
VULN_DB_DOWNLOAD_TIME_LIMIT_SECONDS="120"

# Holds the module cache and the vulnerability database, filled by the networked warm-up below and
# mounted read-only into the check. It persists between runs by design, as a cache; the run itself
# writes nothing to it. `docker volume rm yappblocker-check-cache` resets it.
CACHE_VOLUME_NAME="yappblocker-check-cache"

# govulncheck's database as one archive, served by the same host govulncheck queries by default
VULN_DB_ARCHIVE_URL="https://vuln.go.dev/vulndb.zip"

# Set when this script has re-run itself under `sg docker`, so a second failure stops instead of looping
SG_REEXEC_GUARD_ENV_VAR="YAPPBLOCKER_CHECK_IN_CONTAINER_SG"

# A process started before its user joined the docker group (an agent server that predates the
# grant) does not carry the group. If the user is a member on paper, re-run under `sg docker`,
# which grants it to this process alone without a restart or a password.
if ! docker info >/dev/null 2>&1; then
    if [[ -z "${!SG_REEXEC_GUARD_ENV_VAR:-}" ]] \
        && getent group docker | cut -d: -f4 | tr ',' '\n' | grep -qx "$(id -un)"; then
        export "${SG_REEXEC_GUARD_ENV_VAR}=1"
        exec sg docker -c "$(printf '%q ' bash "${script_dirpath}/$(basename "${0}")" "$@")"
    fi
    echo "ERROR: Cannot reach the Docker daemon as $(id -un); 'docker info' fails. Docker must be installed and this user in the docker group." >&2
    exit 1
fi

# The image is derived from the toolchain pin in src/go.mod rather than pinned a second time: the
# golang image of that exact version carries the pinned Go plus the gcc that -race needs on Linux
# (Linear KEV-367). If the two ever disagree, the check's own version assertion fails the run.
pinned_go_version="$(sed -n 's/^toolchain go\([0-9][0-9.]*\)$/\1/p' "${repo_root_dirpath}/src/go.mod")"
if [[ -z "${pinned_go_version}" ]]; then
    echo "ERROR: Found no 'toolchain goX.Y.Z' line in ${repo_root_dirpath}/src/go.mod to derive the image from" >&2
    exit 1
fi
image="golang:${pinned_go_version}-bookworm"

echo "Warming the module cache and vulnerability database in volume '${CACHE_VOLUME_NAME}' (image ${image}, with network)..."
# Everything the check would otherwise download, fetched with network so the check can run without
# it: each module's dependencies, each pinned tool's, and govulncheck's database. The database is
# fetched fresh on every run so the verdict uses a current copy, as CI's does, into a directory of
# its own that this run alone reads: several agents share the volume, and replacing one shared
# directory would pull it out from under a run that is mid-govulncheck. Copies older than an hour
# belong to no live run (each is capped at TIME_LIMIT_SECONDS) and are pruned. The Go module cache
# needs no such care: the go command is built for concurrent use of it. The script's only stdout
# is the new directory's name; the go command's progress goes to stderr.
# shellcheck disable=SC2016  # expanded inside the container, not here
warm_script='
set -euo pipefail
go -C src mod download
# The check runs `go mod tidy -diff`, which also needs the modules that dependencies import only in
# their own tests; `go mod download` skips those, so let tidy fetch them (it changes no file here).
# An untidy module is for the check to report, with its fix, so tidy is not allowed to fail here
go -C src mod tidy -diff >&2 || true
go -C tools mod tidy -diff >&2 || true
for modfile_filepath in tools/*.mod; do
    go -C tools mod download -modfile="$(basename "${modfile_filepath}")"
done
find /cache -mindepth 1 -maxdepth 1 -name "vulndb.*" -mmin +60 -exec rm -rf {} + || true
vulndb_dirpath="$(mktemp -d /cache/vulndb.XXXXXX)"
curl --fail --silent --show-error --location --max-time "${VULN_DB_DOWNLOAD_TIME_LIMIT_SECONDS}" "${VULN_DB_ARCHIVE_URL}" --output /tmp/vulndb.zip
# The image has no unzip; its Python does, in the standard library
python3 -m zipfile -e /tmp/vulndb.zip "${vulndb_dirpath}"
chmod -R a+rX "${vulndb_dirpath}"
basename "${vulndb_dirpath}"
'
vulndb_dirname="$(
    docker run --rm --init \
        --memory "${MEMORY_LIMIT}" --memory-swap "${MEMORY_LIMIT}" \
        --cpus "${CPU_LIMIT}" --pids-limit "${PIDS_LIMIT}" \
        --volume "${CACHE_VOLUME_NAME}:/cache" \
        --volume "${repo_root_dirpath}:/src:ro" --workdir /src \
        --env GOMODCACHE=/cache/gomod --env GOCACHE=/tmp/gocache --env GOFLAGS=-modcacherw \
        --env VULN_DB_ARCHIVE_URL="${VULN_DB_ARCHIVE_URL}" \
        --env VULN_DB_DOWNLOAD_TIME_LIMIT_SECONDS="${VULN_DB_DOWNLOAD_TIME_LIMIT_SECONDS}" \
        "${image}" timeout --signal=KILL "${WARM_TIME_LIMIT_SECONDS}" bash -c "${warm_script}"
)"
if [[ ! "${vulndb_dirname}" =~ ^vulndb\.[A-Za-z0-9]+$ ]]; then
    echo "ERROR: Expected the warm-up to print its vulnerability database directory's name, but it printed '${vulndb_dirname}'" >&2
    exit 1
fi

echo "Running '${CHECK_COMMAND[*]}' in a throwaway container (memory ${MEMORY_LIMIT}, ${CPU_LIMIT} CPUs, ${PIDS_LIMIT} pids, ${TIME_LIMIT_SECONDS}s)..."
check_exit_code=0
# --rm --init: tini reaps zombies, and the container and its anonymous volumes go when it exits.
# --network none: no egress, and no port the run binds is visible outside it.
# --read-only and --tmpfs: the only writable place is a RAM-backed /tmp that vanishes with the run;
#   exec because go test and go run execute the binaries they build there.
# --memory with an equal --memory-swap: a hard ceiling, with no swap to spill into.
# --user: the caller's uid and primary group (not the docker group an `sg` re-run lends this shell),
#   so anything the run could create carries the caller's ownership.
# --cap-drop and no-new-privileges: a test needs no capability, so it gets none.
# timeout inside the container, not around `docker run`: killing the client does not stop a
#   container, but killing its first process ends its PID namespace, and with it every process the
#   check started.
docker run --rm --init \
    --network none \
    --read-only --tmpfs "/tmp:rw,exec,nosuid,nodev,size=${TMPFS_SIZE}" \
    --memory "${MEMORY_LIMIT}" --memory-swap "${MEMORY_LIMIT}" \
    --cpus "${CPU_LIMIT}" --pids-limit "${PIDS_LIMIT}" \
    --cap-drop ALL --security-opt no-new-privileges \
    --user "$(id -u):$(id -g "$(id -un)")" \
    --volume "${CACHE_VOLUME_NAME}:/cache:ro" \
    --volume "${repo_root_dirpath}:/src:ro" --workdir /src \
    --env HOME=/tmp/home \
    --env GOMODCACHE=/cache/gomod --env GOCACHE=/tmp/gocache --env GOPROXY=off \
    --env GOLANGCI_LINT_CACHE=/tmp/golangci-lint \
    --env YAPPBLOCKER_CHECK_VULN_DB="file:///cache/${vulndb_dirname}" \
    "${image}" timeout --signal=KILL "${TIME_LIMIT_SECONDS}" "${CHECK_COMMAND[@]}" \
    || check_exit_code="${?}"

# 137 is the time ceiling's SIGKILL of the whole run. Hitting the memory ceiling looks different: the
# kernel kills one process, typically a compiler or test binary, and the check fails on its
# "signal: killed" with its usual exit code
if [[ "${check_exit_code}" -eq 137 ]]; then
    echo "ERROR: The check hit its ${TIME_LIMIT_SECONDS}s time ceiling and was killed. Nothing it started survives the container." >&2
fi
exit "${check_exit_code}"
