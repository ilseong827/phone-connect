# PocketRelay Ubuntu 1.0

안드로이드의 내 기록을 받아 저장하고, 로컬 웹과 Dot의 읽기 전용 MCP 도구로 조회하는 Go 실행 파일입니다. 서버 실행에 Python, Go 설치는 필요 없습니다. 웹 녹음 재생에는 FFmpeg, 통화 내용 검색에는 whisper.cpp와 음성 인식 모델이 추가로 필요합니다.

## 설치 및 실행

일반 Intel/AMD 우분투에는 `pocketrelay-linux-amd64.tar.gz`, ARM 우분투에는 `pocketrelay-linux-arm64.tar.gz`를 사용합니다. `uname -m`으로 확인합니다.

```bash
tar -xzf pocketrelay-linux-amd64.tar.gz
cd pocketrelay-linux-amd64
bash install.sh
export PATH="$HOME/.local/bin:$PATH"
sudo apt-get update
sudo apt-get install -y ffmpeg
pocketrelay
pocketrelay --state
pocketrelay --link
```

설치 위치: `~/.local/lib/pocketrelay/pocketrelay`; 명령 링크: `~/.local/bin/pocketrelay`.
데이터: `~/.local/share/pocketrelay/`. 실행 파일을 교체해도 이 폴더는 보존됩니다.
현재 셸 외에도 명령을 쓰려면 `~/.profile`의 PATH에 `~/.local/bin`을 추가하거나 위 절대 경로로 실행합니다.

같은 우분투 컴퓨터의 브라우저에서 **http://localhost:8788**을 엽니다. Dot 컴퓨터에 설치했다면 Dot의 컴퓨터 브라우저에서 이 주소를 엽니다. 사용자 Windows의 localhost는 다른 컴퓨터이므로 동일 주소로 Dot 서버를 볼 수 있는 것은 아닙니다. 원격 PC에서 필요할 때는 SSH 로컬 포워딩 등 별도 연결이 필요합니다.

## 휴대폰 연결

웹 화면과 MCP는 루프백에만 바인딩됩니다. 휴대폰에서는 우분투 서버의 localhost에 접근할 수 없으므로 **수신 포트 8787만 HTTPS로 연결**해야 합니다.

빠른 시험:

```bash
bash install-tunnel.sh
export PATH="$HOME/.local/bin:$PATH"
pocketrelay tunnel on
pocketrelay restart
pocketrelay pair
```

`phoneServerReady: true`가 되면 `phoneServer`를 휴대폰 앱의 서버 주소에, `uploadKey`를 인증키에 넣습니다. 시작 직후 URL 생성에 시간이 걸리면 `pair`를 다시 실행합니다. Quick Tunnel 주소는 재시작 시 달라져 휴대폰 주소도 다시 입력해야 합니다. 장기간에는 고정 HTTPS 도메인을 127.0.0.1:8787에 프록시하고 다음과 같이 설정합니다.

```bash
pocketrelay tunnel off
pocketrelay config --url https://내-수신-도메인
pocketrelay restart
pocketrelay pair
```

`--url`은 안내용 주소를 설정합니다. DNS나 HTTPS 프록시를 자동 생성하지 않습니다. 수신 인증키는 웹/MCP용 비밀번호가 아닙니다. 8788/8789를 공개 프록시에 연결하지 않습니다.

기존 Android APK와 프로토콜이 호환됩니다. APK는 패키지에 포함했으며 `/app.apk`에서도 제공합니다. 앱에서 알림 접근, SMS/연락처 권한, Samsung Voice Recorder 폴더 접근을 사용자 본인이 허용합니다. 처음 가져올 기간을 선택한 뒤 자동 동기화를 켭니다. Android 절전 정책에 따라 전송이 지연될 수 있습니다.

## 부팅 후에도 실행

```bash
pocketrelay install-service
systemctl --user status pocketrelay
loginctl enable-linger "$USER"
```

사용자 systemd 서비스와 linger가 허용되는 우분투 환경에서 로그아웃 후에도 유지됩니다. `enable-linger`가 관리자 권한을 요구하면 해당 환경의 관리 정책을 따라야 합니다. systemd 없는 컨테이너에서는 `pocketrelay`가 백그라운드 실행되지만 호스트 종료·휴면·컨테이너 재생성까지 견디는 것은 아닙니다. 해당 환경의 supervisor에서 `pocketrelay serve`를 실행해야 합니다. Dot 컴퓨터의 systemd, 저장소 수명, 장기 프로세스 허용 여부는 실제 컴퓨터에서 `doctor` 및 재시작 검증이 필요합니다.

서비스 로그: `journalctl --user -u pocketrelay -n 80`. 일반 백그라운드 실행 로그: `pocketrelay logs`.
제거: `pocketrelay uninstall-service` (기록은 삭제하지 않음).

## 통화 녹음 내용 검색

```bash
sudo apt-get install -y ffmpeg cmake g++ git curl
bash install-transcription.sh base
pocketrelay restart
pocketrelay transcribe --all
```

음성은 외부 전사 API로 전송하지 않고 이 서버의 whisper.cpp CPU로 변환합니다. 설치 때 소스와 모델을 다운로드합니다. 기본 multilingual `base` 모델은 약 142MB이며, 한국어 품질이 부족하면 `small`(약 466MB)을 선택할 수 있습니다. 처리 속도와 품질은 CPU·음질에 따라 달라집니다. 기계 전사이므로 정확한 발언·일정은 원음과 대조합니다. 화자 이름을 추측하지 않습니다.

새 녹음은 자동 전사되고, 기존 녹음은 `transcribe --all`로 대기열에 추가합니다. 실패한 개별 파일은 `transcribe --id 기록ID`로 재시도합니다. 웹 녹음 카드의 **통화 내용 / 전사 상태 확인** 버튼에서 결과를 봅니다. 전사가 끝나기 전에는 Dot이 파일 이름만으로 통화 내용을 알 수 없습니다.

## Dot / ChatGPT 연결

서버에는 다음 읽기 전용 MCP 도구가 있습니다.

| 도구 | 기능 |
|---|---|
| search_messages | 카카오톡·문자·연락처 이름 검색 |
| get_conversation | 저장된 특정 방/문자 대화 조회 |
| get_record | 원본 기록, 발신자, 방, 시간, 첨부 메타데이터 조회 |
| list_recordings | 녹음 목록 |
| get_transcript | 녹음 전사 결과/상태 |
| search_transcripts | 완료된 전사 내용 검색 |
| relay_status | 기록 수와 수집 범위 확인 |

Streamable HTTP 엔드포인트는 `http://127.0.0.1:8789/mcp`입니다. **ChatGPT 서버에 localhost URL만 입력해서는 연결되지 않습니다.** 공식 Custom MCP의 **Tunnel / Secure MCP Tunnel** 경로로 이 서버가 실행 중인 컴퓨터에 터널 클라이언트를 연결하고, 제품 화면에서 해당 로컬 MCP 서비스를 선택합니다. 터널 클라이언트 설치·인증은 계정의 공식 안내를 따릅니다. 설치한 커스텀 MCP 플러그인을 Dot에서 활성화하고 `relay_status`, `search_messages`, `get_transcript`를 실제 호출해 확인해야 연결 완료입니다. 이 배포 파일만 설치한다고 계정 플러그인이 자동 등록되지는 않습니다.

stdio MCP를 지원하는 로컬 클라이언트에서는 명령 `~/.local/bin/pocketrelay`, 인수 `mcp`를 사용합니다. HTTP와 stdio 모두 저장된 기록만 읽으며 메시지를 발송하거나 삭제하지 않습니다. 첨부는 메타데이터와 로컬 웹 다운로드로 제공하며 PDF/Word/Excel 본문 추출이나 이미지 OCR을 자동 수행하지 않습니다.

Dot이 연결된 컴퓨터의 명령을 사용할 수 있다면 MCP 등록 전에도 다음과 같이 JSON을 읽을 수 있습니다.

```bash
pocketrelay query --type sms --query '이름 또는 내용' --limit 50
pocketrelay query --type kakao --query '회의'
pocketrelay get --id 기록ID
```

일정을 찾아 달라고 요청하면 Dot이 이 기록과 기존 Google Calendar 플러그인을 함께 사용하도록 합니다. 본 서버가 Google 계정에 자동 연결하거나 캘린더에 무단으로 일정을 쓰지는 않습니다. 정기 확인·알림은 Dot에 별도 반복 작업을 만들고 저장 여부를 확인해야 합니다. 데이터 연결만으로 모니터링 예약이 생기지 않습니다.

공식 근거:

- https://learn.chatgpt.com/docs/dots/computers-and-apps
- https://learn.chatgpt.com/docs/dots/tasks-and-memory
- https://developers.openai.com/api/docs/guides/custom-mcp-server
- https://developers.openai.com/blog/connect-private-mcp-servers-to-openai-products
- https://github.com/ggml-org/whisper.cpp

## 명령 모음

| 명령 | 의미 |
|---|---|
| pocketrelay / start | 백그라운드 시작 |
| stop / restart | 종료 / 재시작 |
| --state / --status / --sate | 상태 (오타 별칭 포함) |
| --link | 웹·수신·MCP 주소 |
| pair | 휴대폰 주소와 인증키 |
| doctor | 프로그램·모델·포트 진단 |
| config | 인증키를 숨긴 설정 조회 |
| logs --lines 80 | 일반 실행 로그 |
| query --type sms --query 텍스트 | 기록 검색 |
| get --id ID | 기록 및 전사 조회 |
| export --out 파일.jsonl | 이벤트 JSONL 내보내기 |
| backup --out 새폴더 | DB·원본·전사 백업 |
| restore --from 백업폴더 | 빈 데이터 폴더로 복원 |
| migrate --from 기존폴더 | 기존 Python 수신 데이터 가져오기 |
| transcribe --all / --id ID | 전사 예약 |
| install-service / uninstall-service | systemd 등록·해제 |
| mcp | stdio MCP |
| serve | supervisor용 전경 실행 |
| --help / --version | 도움말·버전 |

공통 `--data /절대/데이터경로`로 별도 인스턴스를 구성할 수 있습니다. `config --web-port 18788 --ingest-port 18787 --mcp-port 18789`로 포트를 바꾸며 재시작 후 적용됩니다. 시간 필드는 기존 Android의 Unix 밀리초를 유지합니다. 검색의 `--after`는 시간이 아니라 페이지 커서입니다. 결과 `more`가 true면 `cursor`를 다음 `--after`로 넘깁니다.

## 기존 Windows 데이터 이전

현재 Windows 서버와 폰의 전송을 중지한 다음 `receiver-data` 폴더 전체를 우분투로 복사합니다. 실행 중 DB 파일 하나만 복사하면 WAL 기록이 빠질 수 있습니다. 원본 폴더는 보존합니다.

```bash
pocketrelay stop
pocketrelay migrate --from /복사한/receiver-data
pocketrelay
```

대상 데이터 폴더가 비어 있을 때만 가져옵니다. 이미 새 기록을 받은 경우 별도 `--data` 폴더로 가져온 후 해당 폴더를 사용하세요. 병합 기능은 제공하지 않습니다. 예:

```bash
pocketrelay migrate --from /복사한/receiver-data --data "$HOME/.local/share/pocketrelay-import"
pocketrelay start --data "$HOME/.local/share/pocketrelay-import"
```

인증키는 백업·이전에 포함하지 않습니다. 이전 후 `pair` 값으로 휴대폰을 다시 연결합니다. 이미 전송한 기록은 휴대폰이 중복 전송하지 않으므로 예전 서버 데이터가 필요하면 반드시 이전해야 합니다. 백업에는 사적인 메시지와 녹음이 있으므로 개인 저장소에 보관합니다.

## 실제 수집 범위

- 카카오톡: 알림에 제공되는 수신 메시지와 접근 가능한 첨부/미리보기. 공식 개인 대화 API 없이 자신의 발신 카톡·전체 과거 내역·모든 첨부 원본을 백그라운드로 가져올 수는 없습니다.
- SMS/MMS: 권한을 허용한 휴대폰의 송수신, 연락처 이름, 접근 가능한 MMS 첨부. RCS는 보장하지 않습니다.
- 녹음: 사용자가 허용한 Voice Recorder 폴더의 파일. 폴더 밖 통화 녹음은 추가 폴더 선택이 필요합니다.
- GIF·이미지·영상·PDF·Word·Excel 등: 접근 가능한 파일을 원본으로 저장·다운로드합니다. 자동 수집할 수 없는 카톡 파일은 공유 → 내 기록 보내기로 전달합니다. 직접 공유에 원래 방·발신자·원래 전송 시간이 없으면 만들어내지 않습니다.
- 수신 첨부는 파일당 100MiB, 녹음 요청은 512MiB 제한입니다. 브라우저 미지원 영상은 다운로드로 열어야 할 수 있습니다.

서버 바이너리 x86_64는 로컬 WSL 우분투에서 실행 검증하며 arm64는 교차 빌드본입니다. 실제 Dot 계정 연결과 그 컴퓨터의 상시 실행은 배포 후 확인해야 합니다.
