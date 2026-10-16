# 서버

기본 운영 방식은 **[Podman 단일 컨테이너](container/README.md)**입니다. 한 번 생성한 뒤 시작 버튼만 누르면 웹·수신·Dot 조회·터널·자동 전사가 함께 실행됩니다.

- `src/`: Go 수신 서버와 웹 UI, `cmd/phone-connect-reader/` 읽기 전용 조회 API
- `container/`: 자동 시작 스크립트와 운영 안내
- `Containerfile`: 서버·음성 엔진·모델·터널을 포함하는 이미지
- `run.sh`: Podman 시작/중지 도우미
- `legacy-windows/`: 기존 Python 수신 서버. 새 설치에는 컨테이너를 사용합니다.

단독 Linux 바이너리가 필요한 경우 [네이티브 설치 안내](src/README-UBUNTU.md)를 참고합니다. Dot의 명령 샌드박스에는 수신 서버를 설치하지 않고 [외부 조회 클라이언트](../dot/README.md)를 사용합니다.
