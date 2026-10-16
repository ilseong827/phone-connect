# Dot · 외부 HTTPS 조회

Dot에는 서버를 설치하지 않습니다. 로컬 PC의 Podman 컨테이너에 저장된 기록을 조회합니다.

1. PC에서 컨테이너를 시작합니다. 웹 `http://localhost:8788` → **연결 → Dot 연결 파일 다운로드**.
2. 받은 `dot-reader.json`을 Dot에 비공개로 전달합니다. 인증키가 있으므로 공개 저장소에 올리지 않습니다.
3. Dot에 [SETUP.txt](SETUP.txt)의 지시문과 저장소 링크를 전달합니다.
4. Dot이 아래 상태 조회와 실제 기록 조회에 성공하는지 확인합니다.

```bash
python3 client.py --config dot-reader.json relay_status
python3 client.py --config dot-reader.json search_messages --query 일정
python3 client.py --config dot-reader.json list_recordings
python3 client.py --config dot-reader.json get_transcript --id RECORD_ID
```

`client.py`는 Python 표준 라이브러리만 사용하며 HTTP/HTTPS 프록시 환경변수를 따릅니다. 서버·터널·apt·pip 설치가 필요 없습니다. SOCKS 전용 환경은 지원하지 않습니다. 이 도구는 Dot이 명령으로 호출하는 조회 클라이언트이며 계정에 등록된 Gmail 플러그인과 동일한 설치 방식은 아닙니다.

실제 Dot 환경에서의 최종 테스트는 별도입니다. 임시 터널 주소가 바뀌면 웹에서 설정 파일을 다시 받아 전달합니다. 전사 완료 전에는 통화 내용을 추측하지 않습니다.
