#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$(readlink -f "$0")")/.."
repo="${1:?Usage: publish-github.sh OWNER/REPO private|public v1.0.0}"
visibility="${2:-private}"
tag="${3:-v1.0.0}"
[[ "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || { echo 'Invalid repository' >&2; exit 1; }
[[ "$visibility" == private || "$visibility" == public ]] || { echo 'Invalid visibility' >&2; exit 1; }
[[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]] || { echo 'Invalid version tag' >&2; exit 1; }
gh auth status --hostname github.com
for file in dist/pocketrelay-linux-amd64.tar.gz dist/pocketrelay-linux-arm64.tar.gz dist/PocketRelay-debug.apk; do
  [[ -s "$file" ]] || { echo "Missing release file: $file" >&2; exit 1; }
done
[[ -d .git ]] || git init -b main
if git remote get-url origin >/dev/null 2>&1; then
  echo 'An origin already exists. Review it and publish updates manually; nothing overwritten.' >&2; exit 1
fi
login=$(gh api user --jq .login)
uid=$(gh api user --jq .id)
git config user.name "$login"
git config user.email "$uid+$login@users.noreply.github.com"
git add .gitignore .gitattributes .github scripts GITHUB.md README.md VALIDATION.md app/src app/build.gradle gradle gradlew gradlew.bat build.gradle settings.gradle gradle.properties server dot phone-connect.ps1 .containerignore
if git diff --cached --name-only | grep -E '(^|/)(\.tools|receiver-data|local\.properties|config\.json|pc-session\.json)(/|$)|\.(sqlite3|audio|blob|jks|keystore|pem|key)$'; then
  echo 'Private files staged; aborting' >&2; exit 1
fi
if ! git diff --cached --quiet; then git commit -m 'Add PocketRelay Android app and native Linux daemon'; fi
if gh repo view "$repo" --json name >/dev/null 2>&1; then
  gh auth setup-git --hostname github.com
  refs=$(git ls-remote "https://github.com/$repo.git")
  [[ -z "$refs" ]] || { echo 'Remote is not empty; refusing to overwrite history' >&2; exit 1; }
else
  gh repo create "$repo" "--$visibility" --description 'Connect your messages and call recordings to Dot & Web'
fi
gh auth setup-git --hostname github.com
git remote add origin "https://github.com/$repo.git"
git push -u origin HEAD:main
git tag "$tag"
git push origin "$tag"
(cd dist; sha256sum pocketrelay-linux-amd64.tar.gz pocketrelay-linux-arm64.tar.gz PocketRelay-debug.apk > SHA256SUMS)
gh release create "$tag" dist/pocketrelay-linux-amd64.tar.gz dist/pocketrelay-linux-arm64.tar.gz dist/PocketRelay-debug.apk dist/SHA256SUMS --repo "$repo" --verify-tag --title "Phone Connect $tag" --notes-file scripts/release-notes.md
gh repo view "$repo" --json url --jq .url
