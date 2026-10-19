# Google 로그인으로 외부 웹 조회

v1.2.2는 로컬 관리 화면과 별도로 포트 8791에 인증된 읽기 전용 웹을 제공합니다. Cloudflare Access 가입이나 결제수단 등록은 필요하지 않습니다. 기존 Cloudflare Tunnel이 HTTPS 트래픽을 전달하고 Auth0가 Google 로그인을 처리합니다.

## 설정

1. Auth0에서 웹 전용 public OAuth client(Single Page Application, token endpoint authentication `none`)를 만듭니다. Google 연결을 활성화하고 콜백은 `https://phoneview.example.com/auth/callback`, 로그인 URI는 `https://phoneview.example.com/auth/login`으로 정확히 지정합니다. 와일드카드는 사용하지 않습니다. 기존 ChatGPT 클라이언트 설정을 변경할 필요는 없습니다.
2. 기존 `/data/mcp.json`의 issuer와 ownerSubject를 사용합니다. Google 로그인 subject가 정확히 일치해야 하며 이메일 일치만으로 허용하지 않습니다.
3. `/data/web.json`을 준비합니다.

```json
{"publicUrl":"https://phoneview.example.com","clientId":"YOUR_WEB_CLIENT_ID"}
```

4. 기존 named tunnel 설정의 마지막 404 규칙 앞에 추가하고 해당 hostname의 DNS를 터널에 연결합니다.

```yaml
  - hostname: phoneview.example.com
    service: http://127.0.0.1:8791
```

5. 컨테이너를 재시작합니다. 8791을 호스트에 공개하거나 Host 헤더를 localhost로 덮어쓰지 않습니다. Auth0의 공개 클라이언트 ID만 필요하며 Client Secret은 읽거나 저장하지 않습니다. Google의 개발용 연결 키를 사용하고 있다면 장기 운영 전 Auth0 안내에 따라 본인 Google OAuth 자격증명으로 전환하세요.

## 사용과 범위

주소를 열어 Google 계정을 선택하면 문자·카톡·녹음·첨부를 확인할 수 있습니다. 카톡은 저장된 수신 알림만 제공합니다. 로컬 관리 화면은 기존 localhost 주소에서 사용합니다. 외부에서는 인증키 다운로드, QR 연결 정보, MCP 설정, 서버 제어 API를 제공하지 않습니다.

로그인은 Authorization Code + PKCE(S256), state, nonce와 ID token의 서명·issuer·audience·만료·ownerSubject 검증을 사용합니다. 로그인 토큰을 브라우저 저장소에 보관하지 않으며, 브라우저에는 Secure/HttpOnly/SameSite 쿠키의 무작위 세션 ID만 전달합니다. 세션은 최대 8시간 또는 ID token 만료까지 유지됩니다. 컨테이너 재시작 시 다시 로그인하며 자동 로그인 화면 이동이 지원됩니다. 로그아웃은 이 웹 세션을 폐기하며 Google 계정 자체를 로그아웃하지 않습니다.

MCP 조회를 비활성화하거나 ownerSubject/issuer를 변경하면 웹 접근도 차단됩니다. 새 owner/issuer 적용에는 컨테이너 재시작이 필요합니다. 새 PC 이전 시 `web.json`과 터널 설정을 함께 복원하세요. 같은 터널을 다른 데이터 서버 두 곳에서 동시에 실행하지 않습니다.

## 검증

`go test ./...`에서 로그인 state/PKCE, 잘못된 owner·nonce·audience, 만료 토큰, callback 재사용, 인증 없는 조회, Host/Origin 위조, 설정 API 접근, 로그아웃 세션 폐기 및 접근 비활성화를 검사합니다.

2026-10-19 실제 Google 로그인 후 외부 웹에서 카톡 목록과 테스트 녹음 재생·전사를 확인했습니다. 비인증 공개 API 요청은 401, 휴대폰 수신 health는 정상 응답했습니다. 모바일 브라우저 실기기와 장기 세션 유지 시험은 별도입니다.
