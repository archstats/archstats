#!/usr/bin/env bash

# new-adr.sh - Automatically generate a new Architecture Decision Record (ADR)
# Usage: ./scripts/new-adr.sh "Title of Decision"

set -euo pipefail

# Ensure script is run from project root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$PROJECT_ROOT"

ADR_DIR="docs/adr"
TEMPLATE="$ADR_DIR/template.md"
INDEX="$ADR_DIR/README.md"

if [ $# -lt 1 ]; then
    echo "❌ Error: Missing ADR Title."
    echo "Usage: $0 \"Title of Your Decision\""
    exit 1
fi

TITLE="$1"

# Verify template exists
if [ ! -f "$TEMPLATE" ]; then
    echo "❌ Error: Template not found at $TEMPLATE"
    exit 1
fi

# Determine next number
MAX_NUM=0
if [ -d "$ADR_DIR" ]; then
    for f in "$ADR_DIR"/[0-9][0-9][0-9][0-9]-*.md; do
        if [ -f "$f" ]; then
            filename=$(basename "$f")
            num=$(echo "$filename" | cut -d'-' -f1)
            # Remove leading zeros to treat as base 10 integer
            num_base10=$((10#$num))
            if [ "$num_base10" -gt "$MAX_NUM" ]; then
                MAX_NUM="$num_base10"
            fi
        fi
    done
fi

NEXT_NUM=$((MAX_NUM + 1))
PADDED_NUM=$(printf "%04d" "$NEXT_NUM")

# Generate kebab-case slug for filename
SLUG=$(echo "$TITLE" | tr '[:upper:]' '[:lower:]' | sed -e 's/[^a-z0-9]/-/g' -e 's/-\+/-/g' -e 's/^-//' -e 's/-$//')
FILENAME="$PADDED_NUM-$SLUG.md"
TARGET_PATH="$ADR_DIR/$FILENAME"
CURRENT_DATE=$(date +"%Y-%m-%d")

# Create the ADR file from template, substituting fields
echo "📝 Creating new ADR: $TARGET_PATH"
sed -e "s/^# \[Number\]\. \[Title\]/# $PADDED_NUM. $TITLE/" \
    -e "s/- \*\*Status\*\*: \[- \*\*Status\*\*: .*/- **Status**: Proposed/" \
    -e "s/\[Status: .*/Proposed/" \
    -e "s/\[Date: .*/$CURRENT_DATE]/" \
    "$TEMPLATE" > "$TARGET_PATH"

# Pre-fill date properly in status section
sed -i.bak -e "s/- \*\*Date\*\*: .*/- **Date**: $CURRENT_DATE/" "$TARGET_PATH" && rm -f "${TARGET_PATH}.bak"

# Append to the README.md index table if it exists
if [ -f "$INDEX" ]; then
    echo "🔗 Appending to index: $INDEX"
    echo "| $PADDED_NUM | [$TITLE]($FILENAME) | Proposed | $CURRENT_DATE |" >> "$INDEX"
fi

echo "✅ ADR successfully created!"
echo "👉 File: $TARGET_PATH"
