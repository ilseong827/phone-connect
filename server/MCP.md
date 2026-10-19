# ChatGPT · Dot 원격 MCP 연결

Phone Connect의 수신·저장·전사는 본인 PC의 Podman 컨테이너에서 실행됩니다. ChatGPT에는 원격 MCP를 **계정용 플러그인**으로 연결합니다. Dot에서 Python이나 별도 수신 서버를 실행하지 않습니다.

구현 상태: OAuth 보호 리소스·JWT 검증·공식 Go MCP SDK·로컬 설정 화면을 구현했습니다. 가짜 데이터와 테스트 서명키로 로컬 검증했습니다. **실제 인증 제공자 로그인, 사용자 ChatGPT 계정 연결, Dot/모바일 조회는 아직 검증하지 않았습니다.**

## 연결 구조

```text
휴대폰 → 업로드 서버 8787 → PC의 /data 바인드 마운트
ChatGPT/Dot → HTTPS /mcp → 조회 서버 8790 → 읽기 전용 DB
PC 사용자 → localhost:8788/mcp.html → 연결 설정
ChatGPT OAuth → 인증 제공자 로그인/승인 → 짧은 수명의 API 접근 토큰
```

조회 도구: `relay_status`, `search_messages`, `get_conversation`, `get_record`, `list_recordings`, `get_transcript`, `search_transcripts`, `get_profile`.
MCP로는 메시지 전송, 삭제, 서버 설정 변경을 할 수 없습니다. 녹음 내용은 전사 텍스트로 제공합니다. 원본 녹음을 MCP 응답에 넣거나 자동으로 새 메시지 알림을 보내는 기능은 이번 구현에 없습니다.

## 사용자가 준비할 것

1. **고정 HTTPS 주소** 하나. 예: `https://records.example.com`. localhost 주소는 ChatGPT 클라우드에서 직접 접근할 수 없습니다.
2. **OAuth/OIDC 인증 제공자**. 아래는 Auth0를 기준으로 한 설정입니다. 서버는 RS256 JWT 접근 토큰, OIDC discovery, HTTPS JWKS, PKCE S256을 지원하는 제공자와 연결됩니다. 비밀번호와 토큰 발급/갱신은 해당 제공자가 처리합니다.
3. 마지막에 ChatGPT 웹에서 사용자 지정 MCP 플러그인을 추가하고 로그인·승인합니다.

가입·도메인 구매·요금제 선택은 자동으로 수행하지 않습니다. 서비스 가격과 계정별 기능 제공 여부를 직접 확인해야 합니다. OpenAI Secure MCP Tunnel은 별도 권한/API 키가 필요한 대안이며 이번 컨테이너에 그 클라이언트는 포함하지 않았습니다.

## 1. 고정 주소를 컨테이너에 연결

기존 HTTPS 리버스 프록시가 있으면 조회 서버 **8790**으로 연결하세요. 공개 Host 헤더는 설정한 주소와 같아야 합니다. 관리 웹 **8788**과 기존 로컬 MCP **8789**를 공개하지 않습니다.

Cloudflare의 remotely managed tunnel을 사용할 경우:

1. 본인 Cloudflare 계정에서 터널과 고정 호스트 이름을 생성합니다.
2. 해당 호스트의 Service를 `http://127.0.0.1:8790`으로 지정합니다. cloudflared는 조회 서버와 같은 컨테이너에서 실행됩니다. HTTP Host Header를 localhost로 덮어쓰지 않습니다.
3. 터널 토큰만 PC의 바인드 마운트 폴더에 `reader-tunnel-token`이라는 파일로 비공개 저장합니다. 토큰을 GitHub·채팅에 넣지 않습니다.
4. 컨테이너를 재시작합니다. 시작 스크립트가 이 파일을 감지하여 `--token-file`로 고정 터널을 시작합니다. 파일이 없으면 기존 임시 터널을 사용합니다.

휴대폰 업로드용 터널은 기존대로 유지됩니다. 이 토큰은 MCP 조회용입니다. 임시 URL로도 시험할 수 있지만 주소가 바뀌면 아래 API 식별자·MCP 설정·ChatGPT 연결을 모두 변경해야 하므로 상시 운용에는 고정 주소를 사용하세요.

## 2. OAuth 제공자 설정 — Auth0 예시

1. 본인 Auth0 계정/tenant에서 **API**를 생성합니다.
   - 이름: `Phone Connect`
   - Identifier: `https://records.example.com/mcp` — 아래 MCP 주소와 정확히 동일
   - Signing Algorithm: `RS256`
   - Permission: `records:read`
   - 접근 토큰 수명은 짧게 설정하고, 필요한 경우 제공자의 Offline Access/Refresh Token 기능을 활성화합니다.
2. Tenant Settings → Advanced에서 **Resource Parameter Compatibility Profile**을 활성화합니다. ChatGPT의 `resource`가 API audience로 처리되어야 합니다.
3. ChatGPT 연결용 OAuth application을 준비합니다. Authorization Code + PKCE(S256), 필요한 경우 Refresh Token을 허용합니다. 본인이 로그인할 연결을 활성화합니다.
4. ChatGPT 등록 화면에서 표시하는 **정확한 콜백 URL**을 application의 Allowed Callback URLs에 추가합니다. 추측한 URL이나 와일드카드를 사용하지 않습니다.
5. 정적 OAuth client 방식이면 Client ID/Client Secret을 ChatGPT의 비공개 연결 화면에 입력합니다. 이 값은 Phone Connect 설정이나 저장소에 넣지 않습니다. 제공자에서 CIMD/DCR을 준비한 경우 해당 등록 방식도 사용할 수 있습니다.
6. 본인 로그인 계정의 **User ID (`sub`)**를 확인합니다. 예: `auth0|...`, `google-oauth2|...`. 다른 사용자가 tenant에 가입하더라도 서버는 이 ID와 일치하지 않으면 거절합니다.

공급자 설정은 계정별로 다를 수 있습니다. 실제 provider의 discovery에 `code_challenge_methods_supported: ["S256"]`가 있어야 합니다. API audience 대신 OAuth application의 Client ID를 넣거나 ID token을 보내면 거절됩니다. API 접근 토큰에 `records:read` scope가 필요합니다.

## 3. PC 웹에서 저장

`http://localhost:8788` → **연결 → MCP 연결 설정 열기**.

- 공개 HTTPS 기본 주소: `https://records.example.com`
- Issuer: Auth0가 제공하는 정확한 tenant issuer URL, 보통 `https://TENANT.auth0.com/`
- 본인 Subject: 앞서 확인한 User ID
- MCP 조회 활성화 → 저장

설정은 바인드 마운트의 `mcp.json`에 보관됩니다. 저장 후 서버 재시작 없이 적용됩니다. 잘못된 설정은 접근을 허용하지 않습니다. 활성화 해제 시 이후 MCP 요청은 즉시 거절됩니다. 이미 반환된 데이터는 회수되지 않습니다.

## 4. ChatGPT 계정용 플러그인으로 연결

ChatGPT **웹의 계정용 Plugins → Add custom MCP server** 경로를 사용합니다. 데스크톱의 STDIO/환경변수 입력 화면은 로컬 MCP 설정이며 계정 전체 연결과 다릅니다.

- Name: `Phone Connect`
- URL: `https://records.example.com/mcp`
- Authentication: `OAuth`
- 제공자가 요구하는 OAuth client 정보 입력 → 본인 로그인 → 조회 승인
- 만들어진 플러그인을 설치·활성화

해당 메뉴/기능은 계정과 워크스페이스 정책에 따라 다릅니다. UI에 없는 기능이 있다고 가정하지 않습니다. 초기 연결 후 도구 목록에 8개 도구가 나타나는지 확인합니다. 계정용 연결이 실제로 되기 전에는 Dot·모바일 연동 완료로 표시하지 않습니다.

## 검증 순서

1. 인증 없이 `/mcp` 호출: `401`과 OAuth discovery challenge.
2. 인증 후 `relay_status`: 정상 응답.
3. 가짜 문자 1건을 업로드하고 `search_messages`로 ID·본문 일치 확인.
4. 테스트 녹음 전사를 `get_transcript`로 조회.
5. 본인 이외 계정, 만료 토큰, 다른 audience, 잘못된 서명, scope 없는 토큰 거절 확인.
6. 컨테이너 재시작 후 데이터·설정·고정 주소 유지 확인.
7. Dot과 휴대폰에서 동일 플러그인으로 테스트 기록을 조회.

등록이나 도구 목록 표시만 성공한 것은 전체 연동 성공이 아닙니다. 실제 OAuth 제공자에서 refresh/revoke 동작도 확인합니다. 자체 검증하는 JWT는 제공자가 폐기한 뒤에도 토큰 만료까지 유효할 수 있으므로 짧은 수명을 사용하고 긴급 차단에는 MCP 활성화를 해제합니다.

## 실제 연결에서 확인한 문제 해결

- `Client is not authorized to access resource server`: Auth0의 해당 API → Application Access에서 ChatGPT용 application의 **User-delegated Access**에 `records:read`를 허용합니다. Client Access나 Anonymous Access를 열 필요는 없습니다.
- 인증은 성공했지만 도구 검색이 실패하고 서버 로그가 `stage=owner_check`인 경우: 실제 로그인한 계정의 User ID와 서버의 본인 Subject를 비교합니다. 이메일이 같아도 이메일·비밀번호 로그인(`auth0|...`)과 Google 로그인(`google-oauth2|...`)은 서로 다른 계정 ID일 수 있습니다. 본인이 사용할 로그인 계정의 정확한 ID로 설정하고 재연결합니다. 이메일 일치만으로 접근을 허용하거나 소유자 검증을 끄지 않습니다.
- ChatGPT 연결 완료 후 `relay_status`, `search_messages`, `list_recordings`를 실제 호출해 확인합니다. 로컬 STDIO MCP 성공을 원격 OAuth 플러그인 성공으로 대신하지 않습니다.
- 다른 PC로 이전할 때는 이미지 외에 비공개 설정도 복원해야 합니다. `/data/mcp.json`과 Cloudflare 터널 자격증명 등 개인 설정은 공개 저장소·이미지에 포함하지 않습니다. 소유자 설정을 변경했다면 개인 백업도 갱신합니다.

2026-10-19에 고정 도메인·Auth0 OAuth를 사용하는 ChatGPT 웹 계정 플러그인에서 상태, 메시지 및 녹음 목록 조회를 확인했습니다. 모바일·Dot 각각의 실제 조회, 장기 상시 운영, 토큰 refresh/revoke는 별도 검증 대상입니다.

## 개발 테스트 실행

```bash
cd server/src
go test ./...
go test -race ./cmd/phone-connect-reader
go vet ./...
```

테스트는 가짜 SMS·임시 SQLite·테스트 RSA 키만 사용합니다. HTTPS discovery/JWKS, 서명·issuer·audience·subject·만료·scope 검사, 공식 SDK 클라이언트 조회, 호스트/Origin 차단, 설정 저장·비활성화·재로딩을 확인합니다.

## 공식 근거

- [OpenAI MCP 인증](https://developers.openai.com/plugins/build/auth)
- [MCP 플러그인 연결](https://developers.openai.com/api/docs/guides/custom-mcp-server)
- [Dot 플러그인 사용](https://learn.chatgpt.com/docs/dots/computers-and-apps)
- [로컬 MCP와 계정용 연결의 차이](https://learn.chatgpt.com/docs/extend/mcp)
- [Auth0 resource/audience 호환 설정](https://support.auth0.com/center/s/article/mcp-audience-error-with-auth0)
- [Cloudflare token-file](https://developers.cloudflare.com/tunnel/reference/run-parameters/)
