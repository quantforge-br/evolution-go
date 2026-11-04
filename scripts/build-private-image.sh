#!/bin/sh
# Build the private image from the committed "private" branch only.
#
#   scripts/build-private-image.sh [--dry-run] [--load HOST] [TAG]
#
# The build context is "git archive HEAD", so uncommitted or untracked files
# (including .env files) can never reach the image. TAG defaults to
# private-<yyyymmddHHMM>; the image is evolution-go-private:TAG and carries
# the source commit as the org.opencontainers.image.revision label.
# --load HOST streams the image to a Docker host over SSH (docker load).
# Deploying (EVOLUTION_IMAGE + redeploy + /instance/connect) stays manual;
# see SELF_HOSTING.md.
set -eu
cd "$(dirname "$0")/.."

dry_run=0
load_host=
tag=
while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) dry_run=1 ;;
        --load) shift; load_host=${1:?--load needs a host} ;;
        -h|--help) sed -n '2,13p' "$0"; exit 0 ;;
        -*) printf 'unknown option: %s\n' "$1" >&2; exit 2 ;;
        *) tag=$1 ;;
    esac
    shift
done

branch=$(git rev-parse --abbrev-ref HEAD)
if [ "$branch" != private ]; then
    printf 'refusing to build: on branch %s, not private\n' "$branch" >&2
    exit 1
fi
if [ -n "$(git status --porcelain --untracked-files=no)" ]; then
    printf '%s\n' 'refusing to build: tracked files have uncommitted changes' >&2
    exit 1
fi

commit=$(git rev-parse HEAD)
version=$(git show HEAD:VERSION)
tag=${tag:-private-$(date +%Y%m%d%H%M)}
image=evolution-go-private:$tag

printf 'commit:  %s\nversion: %s\nimage:   %s\n' "$commit" "$version" "$image"
if [ -n "$load_host" ]; then printf 'load to: %s\n' "$load_host"; fi
if [ "$dry_run" = 1 ]; then
    printf '%s\n' 'dry run: nothing built'
    exit 0
fi

git archive --format=tar HEAD | docker build \
    --build-arg "VERSION=$version" \
    --label "org.opencontainers.image.revision=$commit" \
    -t "$image" -
if [ -n "$load_host" ]; then
    docker save "$image" | ssh "$load_host" docker load
fi
printf 'built %s from %s\n' "$image" "$commit"
