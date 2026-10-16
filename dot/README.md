현재 권장 연결은 ChatGPT 계정용 원격 MCP입니다. server/MCP.md를 참고하세요. 아래 Python 방식은 이전 호환용이며 계정 플러그인 연결이 아닙니다.

# Dot 연결 — 파일 하나로 실행

PC 웹의 **연결 → Dot 연결 파일 다운로드 (.py)**에서 `dot-reader.py`를 받으세요.
서버 주소, 읽기 전용 인증키, 조회 코드, 사용법이 모두 들어 있습니다.
이 파일 하나를 Dot에 비공개로 전달하고 실행을 요청하면 됩니다. GitHub에서 코드를 추가로 받을 필요가 없습니다.

```bash
python3 dot-reader.py
python3 dot-reader.py search_messages --query 일정
python3 dot-reader.py list_recordings
python3 dot-reader.py get_transcript --id RECORD_ID
python3 dot-reader.py --help
```

옵션 없이 실행하면 연결 상태를 조회합니다. Python 3.8 이상과 기본 라이브러리만 사용하며 pip/apt 설치는 필요 없습니다.
PC 서버가 켜져 있고 HTTPS 주소에 접근할 수 있어야 합니다. 임시 주소가 바뀌면 파일을 다시 다운로드하세요.
이 파일은 인증키를 포함하므로 공개 저장소나 로그에 올리지 마세요. 인코딩은 암호화가 아닙니다.
HTTP/HTTPS 프록시를 따르며 SOCKS 전용 환경은 지원하지 않습니다.
실제 Dot 환경에서의 최종 연결 검증은 별도입니다. 자동 플러그인 등록이나 상시 실행 기능은 아닙니다.

개발용 `client.py`는 연결 정보가 없는 원본입니다. 기존 방식인 `python3 client.py --config dot-reader.json relay_status`도 지원합니다.
