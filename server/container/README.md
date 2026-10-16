# Phone Connect · Podman 단일 컨테이너

수신 서버, 로컬 웹, Dot 조회 API, FFmpeg, whisper.cpp, Cloudflare 터널을 **컨테이너 하나**에서 실행합니다. Compose는 필요 없습니다. 휴대폰과 Dot은 이 PC의 서버에 각각 HTTPS로 접속합니다. Dot에서 서버를 실행하거나 포트를 공개할 필요가 없습니다.

## 시작

일반 사용자는 이미지를 준비하고 컨테이너를 한 번 만든 뒤 Podman 시작 버튼만 누르면 됩니다. 내부 프로그램은 모두 자동 실행됩니다. 아래 명령은 최초 생성·관리용입니다. 웹 연결 화면에서 휴대폰 QR과 Dot 연결 파일을 받을 수 있습니다. Windows의 이 프로젝트에서는 `./phone-connect.ps1 start`로 WSL Podman을 시작할 수도 있습니다.

```bash
# 소스로 빌드 (또는 아래 릴리스 이미지 로드)
podman build -t localhost/phone-connect:1.2.0 -f server/Containerfile .
mkdir -p phone-connect-data
podman run -d --name phone-connect --restart=unless-stopped \
  -p 127.0.0.1:8788:8788 \
  -v "$(pwd)/phone-connect-data:/data" \
  localhost/phone-connect:1.2.0
```

PC 브라우저: **http://localhost:8788**. 관리 웹은 호스트 루프백에만 공개합니다. 수신 8787, MCP 8789, 읽기 API 8790을 호스트/LAN에 직접 공개하지 않습니다. 컨테이너 내부 두 터널은 수신 API와 조회 API에 각각 연결하며 관리 화면은 연결하지 않습니다.

프로그램·의존성은 이미지 안에 있고 DB·녹음·설정·키는 PC의 `phone-connect-data/` 바인드 마운트 폴더에 있습니다. 컨테이너 교체와 데이터 삭제는 별개입니다. 실제 PC 폴더를 삭제하면 데이터도 삭제됩니다.

## 휴대폰·Dot 연결: 웹에서 진행

컨테이너를 켠 뒤 **http://localhost:8788 → 연결**을 엽니다.

1. 휴대폰 앱의 **서버 → QR로 연결**로 웹의 QR을 찍습니다.
2. **Dot 연결 파일 다운로드**를 눌러 `dot-reader.json`을 받습니다.
3. 파일을 Dot에 비공개로 전달하고 [연결 지시문](../../dot/SETUP.txt)을 사용합니다.

터널은 자동으로 시작합니다. 시작 직후에는 주소 생성까지 잠시 걸립니다. 임시 주소는 컨테이너/터널 재시작 시 바뀔 수 있어 QR과 Dot 설정 파일을 다시 받아야 합니다. 한 컨테이너에서 수신 URL과 조회 URL 두 개를 만드는 구성이며 컨테이너 두 개가 아닙니다.

`dot-reader.json`에는 조회 전용 인증키가 있습니다. 공개 GitHub, 공개 링크, QR 서비스에 올리지 마세요. Dot의 비공개 파일 전달 기능으로 이 파일을 전달하고 `dot/SETUP.txt`의 요청문을 사용합니다. Dot은 공개 저장소에서 `dot/client.py`를 다운로드할 수 있습니다. 이 클라이언트는 Python 표준 라이브러리만 사용하고 HTTP_PROXY/HTTPS_PROXY를 따릅니다. 별도 pip·apt·수신 포트가 필요 없습니다. SOCKS 전용 프록시만 있는 환경은 별도 지원이 필요합니다.

이 경로는 **HTTPS 조회 클라이언트**이며 Gmail처럼 계정에 설치된 플러그인으로 자동 등록되는 것은 아닙니다. Dot에서 `relay_status`, 가짜 메시지 조회, 실제 기록 조회를 실행해야 최종 연결 성공입니다. 실제 Dot 실행 환경의 프록시 통과 여부는 여기 PC 테스트만으로 보장하지 않습니다.

조회 API는 `POST /v1/read/{operation}` JSON이며 `Authorization: Bearer <조회키>`를 사용합니다. `GET /v1/tools`로 목록을 볼 수 있습니다. 업로드키와 조회키는 다릅니다. 조회 API에는 업로드·삭제·설정 변경 기능이 없고 SQLite 연결도 읽기 전용입니다. 연락처는 문자에 이름을 붙이는 데만 사용하고 전체 주소록 동기화 이벤트는 외부 조회에서 제외합니다.

녹음 원본은 `download_audio --id <ID> --out recording.audio`로 받습니다. 전사 전에는 `get_transcript`가 `not_transcribed`를 반환하며 내용을 추측하지 않습니다. 카카오톡 발신 메시지·알림에 없는 과거 대화는 여전히 수집할 수 없습니다.

## 음성 전사

FFmpeg·whisper.cpp와 한국어 지원 base 모델은 이미지에 포함됩니다. 컨테이너 시작 시 자동 전사가 켜집니다. 호스트나 컨테이너 안에서 추가 설치 명령을 실행할 필요가 없습니다. CPU 전사 속도·정확도는 녹음과 PC 성능에 따라 달라집니다.

## 운영

```bash
podman stop phone-connect
podman start phone-connect
podman restart phone-connect
podman exec phone-connect phone-connect-info state
```

Podman의 재시작 정책은 PC 전원·WSL/Podman Machine 실행까지 보장하지 않습니다. 24시간 수신하려면 PC를 켜두고 절전을 막으며 Windows 부팅/로그인 때 WSL과 컨테이너를 시작해야 합니다. Linux 상시 서버에서는 systemd/Quadlet 등 호스트 부팅 시 시작 설정을 추가합니다. 임시 터널을 장기 고정 주소로 전환하는 작업은 별도이며 유료 Cloudflare 플랜만 구입한다고 PC가 계속 실행되지는 않습니다.

기존 Windows 서버가 8788을 쓰면 먼저 데이터를 백업·이전하고 이전 서버를 정지한 후 전환하세요. 두 서버가 동일 데이터 디렉터리를 동시에 쓰게 하지 않습니다. 테스트 컨테이너에는 별도 마운트 폴더를 사용합니다.

## 빌드 없이 이미지 가져오기

Releases에서 `phone-connect-container-amd64.tar.gz`와 `SHA256SUMS`를 받습니다. 일반 Intel/AMD PC용 이미지입니다.

```bash
sha256sum --ignore-missing -c SHA256SUMS
podman load -i phone-connect-container-amd64.tar.gz
```

이후 위 최초 `podman run` 명령으로 PC 폴더를 마운트합니다. 이미지를 Podman GUI로 가져올 경우에도 컨테이너 생성 시 **PC 폴더 → /data**, **127.0.0.1:8788 → 컨테이너 8788**을 지정합니다. 이후 시작 버튼만 누르면 됩니다. ARM 컨테이너는 Containerfile로 해당 아키텍처에서 빌드해야 하며 이 릴리스에서 ARM 컨테이너 실기기 검증은 하지 않았습니다.
## 새 서버로 옮길 때

Android v0.5.0부터 새 업로드 키로 연결하면 선택한 기간의 문자·녹음과 연락처 이름을 다시 동기화합니다. 주소만 바뀌고 키가 같으면 기존 완료 상태를 유지합니다. 이미 새 서버에 연결한 경우 앱의 **서버 → 현재 서버로 다시 동기화**를 누릅니다. 카톡 본문과 공유 원본은 전송 완료 후 앱에서 제거되므로 기존 서버 자료를 별도로 이전해야 합니다.
