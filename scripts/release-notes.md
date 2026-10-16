Phone Connect v1.1.0 · Android v0.4.1

- `pocketrelay --pair`: QR 페이지 링크(pairLink), 실제 서버 주소(phoneServer), 인증키(uploadKey).
- 외부 HTTPS 연결 페이지에서 QR 표시, 앱 열기, 수동 주소/키 복사, APK 다운로드.
- 휴대폰 앱의 QR 스캔으로 PC/Dot 웹페이지의 QR을 직접 읽어 서버 화면 이동 및 주소/키 자동 입력. 확인 후 저장하기 전에는 기존 설정 보존.
- `--check-link`: 외부 HTTPS 접속과 인증키 검증. `config --tunnel-protocol http2`: UDP 대신 TCP 터널 선택.
- 테스트: Go 9개, Android 단위 테스트 2개, 실제 Samsung 휴대폰의 링크 자동 입력 및 외부 HTTPS 인증 성공. 사용자가 실제 카메라 QR 스캔·연결 성공 확인. 실제 Dot 환경은 별도 확인 필요.

Android의 카카오톡 수신 알림, SMS/MMS, 연락처 이름, 녹음을 저장하는 PocketRelay입니다.

- Linux amd64/arm64 네이티브 데몬, 기본 로컬 웹 localhost:8788.
- 읽기 전용 MCP 검색 도구, 선택적 whisper.cpp 로컬 음성 전사.
- 상태·링크·서비스 등록·백업/복원·기존 데이터 이전 명령.
- Android v0.4.1 APK 포함. 기존 앱 위에 업데이트 설치하세요.

설치 패키지에 README-UBUNTU.md와 DOT-SETUP.txt가 있습니다. FFmpeg·음성 모델·GitHub 계정/Dot 연결은 별도 설정이 필요합니다.

카카오톡 발신 메시지·알림에 없는 첨부 원본은 자동 수집하지 못합니다. 실제 개인 기록과 인증키는 이 릴리스에 포함하지 않습니다.
