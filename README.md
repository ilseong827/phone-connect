# Phone Connect

**내 휴대폰의 카카오톡·문자·통화 녹음을 PC에 보관하고 Dot에서 조회합니다.**

[컨테이너 설치 파일](https://github.com/ilseong827/phone-connect/releases/tag/v1.2.3) · [Android APK](https://github.com/ilseong827/phone-connect/releases/tag/v1.2.0) · [Docker / Podman 실행 안내](server/container/README.md) · [ChatGPT · Dot MCP 연결](server/MCP.md) · [Google 로그인 외부 웹](server/REMOTE-WEB.md)

> v1.2.3는 고정 도메인 자동 실행과 본인 Google 로그인으로 보호되는 외부 조회 웹을 지원합니다. 릴리스의 `start.sh`와 이미지 파일을 같은 폴더에 두고 개인 `phone-connect-data/`를 복원한 뒤 `bash start.sh`로 시작합니다. OAuth 제공자·주소의 최초 설정과 실제 ChatGPT/Dot 연결은 계정에서 마쳐야 합니다. 개인 인증 정보는 공개 다운로드에 포함되지 않습니다.

## 구조

```text
app/                 Android 앱 (src/main, src/test, src/androidTest)
server/
  src/               Go 수신 서버·웹·조회 API
  container/         단일 컨테이너 자동 시작·연결 안내
  legacy-windows/    기존 Windows 수신 서버 (이전·호환용)
  Containerfile      FFmpeg·whisper.cpp·모델·터널 포함 이미지
  run.sh             Podman 시작·중지 도우미
dot/                 이전 Python 조회 도구 (호환용)
scripts/             배포 도구
```

휴대폰 → **로컬 PC의 Podman 컨테이너 하나** → 로컬 웹 / Dot의 HTTPS 조회.
Dot의 제한된 샌드박스에 수신 서버를 설치하지 않습니다. Compose는 필요 없습니다.

## 실행

이미지를 준비한 뒤 최초 한 번:

```bash
podman run -d --name phone-connect --restart=unless-stopped \
  -p 127.0.0.1:8788:8788 \
  -v "$(pwd)/phone-connect-data:/data" \
  localhost/phone-connect:1.2.0
```

이후에는 **Podman의 시작 버튼** 또는 `podman start phone-connect`만 사용합니다. 수신·웹·조회 API·터널·자동 전사가 함께 시작됩니다. 컨테이너에 들어가 명령을 입력할 필요가 없습니다.

- PC 웹: **http://localhost:8788**
- 웹 **연결 → 휴대폰 QR**: 앱에서 스캔하여 서버 주소·인증키 입력
- 웹 **연결 → MCP 연결 설정 열기**: [OAuth · ChatGPT 계정 연결 안내](server/MCP.md). 로그인 제공자와 고정 주소 설정 후 연결합니다.
- 데이터: PC의 `phone-connect-data/` 바인드 마운트 폴더. 컨테이너를 삭제해도 PC의 마운트 폴더를 지우지 않으면 기록은 남습니다.

소스 빌드와 Windows/WSL 시작 방법은 [서버 안내](server/container/README.md)를 따릅니다. 컨테이너 안에 음성 도구와 한국어 지원 base 모델을 포함하므로 호스트에서 FFmpeg·빌드 도구를 설치할 필요가 없습니다.

## 운영 조건

PC와 Podman/WSL이 실행 중이어야 수신됩니다. 절전·종료 상태에서는 수신할 수 없습니다. 재시작 정책이 PC 전원이나 Windows 시작을 대신하지는 않습니다.

기본 무료 Quick Tunnel은 재시작 시 URL이 바뀔 수 있습니다. 이 경우 휴대폰 QR을 다시 받고, MCP 공개 주소·OAuth API 식별자·ChatGPT 연결도 갱신해야 합니다. 고정 주소 운영에는 별도 도메인·터널 설정이 필요합니다.

MCP는 OAuth와 본인 계정 검증을 사용합니다. 이전 Python 조회키는 호환용이며 MCP 인증에는 사용할 수 없습니다. 관리 웹과 기존 MCP 포트는 외부 터널에 연결하지 않습니다. 실제 Dot에서 조회 성공 여부는 별도로 확인해야 합니다.

카톡은 수신 알림 기반이며 발신 카톡·전체 과거 대화·모든 첨부 원본을 자동으로 가져오지는 못합니다. SMS/MMS 송수신을 지원하고 RCS는 제외합니다. 녹음 내용은 전사 완료 후 조회되며 음성 인식 오류가 있을 수 있습니다.

## 개발·검증

- Android: `./gradlew :app:assembleDebug :app:lintDebug :app:testDebugUnitTest`
- 서버: `cd server/src && go test ./... && go vet ./...`
- 컨테이너 빌드: `podman build -t localhost/phone-connect:1.2.0 -f server/Containerfile .`
- [검증 이력](VALIDATION.md) · [네이티브 Ubuntu 설치](server/src/README-UBUNTU.md)

소스와 릴리스는 공개입니다. 실제 메시지·녹음·주소록·인증키·개인 설정은 포함하지 않습니다.
