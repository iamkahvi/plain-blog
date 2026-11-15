#!/bin/sh
set -eu

script_dir=$(CDPATH= cd "$(dirname "$0")" && pwd)
: "${BLOG_CONTENT_DIR:?Set BLOG_CONTENT_DIR to the destination}"
rsync -a "$script_dir/../content/" "$BLOG_CONTENT_DIR/"
