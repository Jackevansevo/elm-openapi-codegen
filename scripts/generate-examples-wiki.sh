#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

output="${1:-}"
if [[ -n "$output" ]]; then
  mkdir -p "$(dirname "$output")"
  exec >"$output"
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

examples=(
  "Enum|enum|String enums generate custom Elm union types plus decoders and encoders."
  "Default Values|default|Supported default values are used as decoder fallbacks instead of wrapping the field in Maybe."
  "anyOf and Discriminator|cat-dog|Union schemas generate Elm union types. Discriminators decode by tag when a mapping is provided."
  "allOf|all-of|Object schemas composed with allOf are flattened into one generated record."
  "Recursion|recursion|Recursive schemas generate recursive Elm types and lazy decoders."
)

extract_schemas() {
  local input="$1"
  awk '
    /^components:/ {
      in_components = 1
      next
    }
    in_components && /^  schemas:/ {
      in_schemas = 1
      next
    }
    in_schemas {
      if ($0 ~ /^[^[:space:]]/ && $0 != "") {
        exit
      }
      if ($0 ~ /^    /) {
        sub(/^    /, "")
        print
      } else if ($0 == "") {
        print
      }
    }
  ' "$input"
}

generate_elm() {
  local fixture="$1"
  local input="testdata/examples/$fixture/input.yaml"
  local project_dir="$tmp_dir/$fixture"
  local out_dir="$project_dir/src/Generated"

  mkdir -p "$project_dir"
  printf '{"type":"application","source-directories":["src"]}\n' >"$project_dir/elm.json"
  mkdir -p "$out_dir"
  go run ./cmd/elm-openapi-codegen --out "$out_dir" "$input" >/dev/null
  find "$out_dir" -type f -name '*.elm' | sort
}

cat <<'MARKDOWN'
# Examples

These examples are generated from the OpenAPI documents in `testdata/examples`.
Each schema block is the `components.schemas` fragment from the source document,
followed by the Elm modules generated for that example.

MARKDOWN

for example in "${examples[@]}"; do
  IFS='|' read -r title fixture description <<<"$example"
  input="testdata/examples/$fixture/input.yaml"

  if [[ ! -f "$input" ]]; then
    echo "missing example input: $input" >&2
    exit 1
  fi

  echo "## $title"
  echo
  echo "$description"
  echo
  echo '```yaml'
  extract_schemas "$input"
  echo '```'
  echo

  mapfile -t elm_files < <(generate_elm "$fixture")
  if [[ "${#elm_files[@]}" -eq 0 ]]; then
    echo "no Elm files generated for $fixture" >&2
    exit 1
  fi

  for elm_file in "${elm_files[@]}"; do
    module_path="${elm_file#"$tmp_dir/$fixture/"}"
    echo "\`$module_path\`"
    echo
    echo '```elm'
    sed -n '1,$p' "$elm_file"
    echo '```'
    echo
  done
done
