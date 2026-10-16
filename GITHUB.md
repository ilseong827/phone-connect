# Phone Connect · GitHub에서 보관하고 다시 설치하기

소스는 Git 저장소에, 설치 파일은 GitHub Releases에 보관합니다. GitHub는 배포 파일 보관소이며 PocketRelay 서버를 상시 실행해 주는 호스팅은 아닙니다. 실제 수신 서버는 우분투/Dot 컴퓨터에서 실행합니다.

개인용은 비공개 저장소를 기본으로 권장합니다. 비공개 저장소는 다른 PC에서도 해당 GitHub 계정으로 로그인하면 소스와 릴리스를 받을 수 있습니다. 공개 저장소를 선택하면 소스와 배포 파일을 누구나 받을 수 있습니다.

## 업로드 대상

- Android/Go/기존 PC 서버 소스, 설치 안내, 테스트, 빌드 검사 workflow.
- Releases: Linux amd64/arm64 패키지와 기존 서명 Android APK, SHA256 체크섬.
- 제외: 메시지·녹음·주소록 DB, 실제 첨부, 인증키, `.tools`, `receiver-data`, 로컬 설정, 서명 키.

## 새 컴퓨터에서 설치

GitHub 저장소의 Releases에서 아키텍처에 맞는 `pocketrelay-linux-amd64.tar.gz` 또는 `pocketrelay-linux-arm64.tar.gz`를 받습니다. 아래 ilseong827/phone-connect를 실제 저장소로 바꿉니다.

```bash
gh auth login
gh release download --repo ilseong827/phone-connect --pattern 'pocketrelay-linux-amd64.tar.gz' --pattern SHA256SUMS --dir pocketrelay-download
cd pocketrelay-download
sha256sum --ignore-missing -c SHA256SUMS
tar -xzf pocketrelay-linux-amd64.tar.gz
cd pocketrelay-linux-amd64
bash install.sh
export PATH="$HOME/.local/bin:$PATH"
pocketrelay
```

상세 설정은 [Ubuntu 안내](daemon/README-UBUNTU.md)를 따릅니다. 설치 파일을 어디서든 받을 수 있는 것과 기존 기록이 따라오는 것은 다릅니다. 기록은 `backup`/`restore`로 별도 이전합니다.

## 최초 게시

현재 PC의 WSL에서 GitHub CLI로 로그인한 후 실행합니다. 인증 토큰을 소스나 채팅에 붙여 넣지 않습니다.

```bash
gh auth login --hostname github.com --git-protocol https --web --scopes workflow
bash scripts/publish-github.sh ilseong827/phone-connect private v1.0.0
```

이 스크립트는 새 빈 저장소 게시용입니다. GitHub 계정의 이름과 비공개 noreply 이메일을 로컬 커밋에 사용합니다. 기존 커밋이 있는 원격 저장소는 덮어쓰지 않습니다. 기존 릴리스 파일도 덮어쓰지 않습니다.

CI는 소스 푸시 때 Go 테스트·검사·두 아키텍처 빌드와 Android 컴파일·lint를 실행합니다. 자동 생성한 debug APK는 서명 키가 달라 기존 폰에 덮어쓸 수 있으므로 CI에서 배포하지 않습니다. 현재 배포본은 기존 폰과 같은 서명의 APK를 사용합니다. 장기 업데이트용 서명 키는 별도 안전한 백업이 필요합니다.
