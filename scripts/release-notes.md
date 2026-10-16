Phone Connect v1.2.0 · Android v0.5.0

- Podman 컨테이너 하나로 수신 서버, 로컬 웹, 조회 API, 두 HTTPS 터널, FFmpeg, whisper.cpp와 한국어 지원 base 모델을 자동 실행합니다. Compose나 컨테이너 내부 수동 실행 명령이 필요 없습니다.
- PC의 실제 폴더를 /data에 바인드 마운트하여 DB·녹음·첨부·전사·설정·인증키를 유지합니다. 로컬 웹은 localhost:8788입니다.
- 웹 연결 화면에서 휴대폰 QR과 Dot 조회 설정 파일을 받습니다. Dot은 dot/client.py로 외부 HTTPS 조회만 수행합니다. 업로드/조회 키 분리, 읽기 전용 DB 연결, 관리 경로 외부 차단을 적용했습니다.
- Android: 새 서버 키로 연결하면 선택한 기간의 문자·녹음 재동기화. 이미 변경한 경우 서버 → 현재 서버로 다시 동기화. 대기 자료는 보존하고 후속 전송 배치를 예약합니다. 과거 카톡·공유 원본은 기존 서버에서 이전해야 합니다.
- 저장소 구조: app/, server/, dot/. 기존 Windows 구현은 server/legacy-windows/에 보존합니다.

검증: Podman rootless 실제 외부 업로드/조회, 중복 방지, 키 권한 분리, 컨테이너 재시작 후 바인드 데이터·키 유지, 공개 음성 샘플 재생 변환·자동 전사·원본 다운로드 일치. Android 빌드/lint/단위 테스트와 실제 삼성 기기 재동기화 대기열 보존 테스트 통과.

실제 Dot 계정에서의 조회 및 새 APK의 사용자 문자·녹음 재전송 완료는 별도 확인이 필요합니다. Quick Tunnel 주소는 재시작 시 바뀔 수 있습니다. PC가 켜지고 Podman/WSL이 실행돼 있어야 수신됩니다. 개인 데이터와 인증키는 이미지·소스·릴리스에 포함하지 않습니다.

이미지: phone-connect-container-amd64.tar.gz → podman load -i 파일명.
자세한 사용법은 README.md 및 server/container/README.md, Dot 지시문은 dot/SETUP.txt를 확인하세요.