# TripWand Backend API 테스트 가이드

TripWand Backend API를 테스트하기 위한 스크립트 모음입니다.

## 📁 디렉토리 구조

```
test/
├── api/                    # API 테스트 스크립트
│   ├── test_all.sh        # 전체 API 테스트
│   ├── test_travel.sh     # 여행 일정 생성 테스트
│   ├── test_health.sh     # 헬스체크 테스트
│   └── load_test.sh       # 부하 테스트
├── data/                  # 테스트 데이터
│   ├── test_requests.json # 다양한 테스트 요청 데이터
│   └── test_request.json  # 부하 테스트용 단일 요청 (자동 생성)
├── logs/                  # 테스트 로그 (자동 생성)
└── README.md             # 이 파일
```

## 🚀 사용법

### 1. 서버 시작

먼저 TripWand Backend 서버를 시작하세요:

```bash
# 프로젝트 루트에서
go run cmd/main.go

# 또는 빌드 후 실행
go build -o tripwand-backend cmd/main.go
./tripwand-backend
```

### 2. 테스트 스크립트 실행 권한 부여

```bash
cd test/api
chmod +x *.sh
```

### 3. 기본 테스트

#### 헬스체크 테스트
```bash
./test_health.sh
```

#### 여행 일정 생성 테스트 (기본값: 서울 2일)
```bash
./test_travel.sh
```

#### 특정 목적지 테스트
```bash
./test_travel.sh 부산 3
./test_travel.sh 제주도 4
```

#### 전체 API 테스트 (권장)
```bash
./test_all.sh
```

### 4. 부하 테스트

#### 기본 부하 테스트 (동시 5개 요청, 총 20개)
```bash
./load_test.sh
```

#### 커스텀 부하 테스트
```bash
./load_test.sh 10 50  # 동시 10개 요청, 총 50개
```

## 📊 테스트 결과 확인

### 로그 파일

모든 테스트 결과는 `logs/` 디렉토리에 타임스탬프와 함께 저장됩니다:

```bash
# 최신 로그 확인
ls -la logs/

# 특정 테스트 로그 확인
cat logs/20240820_143022_health_check.log
cat logs/20240820_143022_travel_부산_3days.log

# 전체 테스트 로그 확인
cat logs/20240820_143022_*.log
```

### 실시간 모니터링

서버 로그를 실시간으로 확인하려면:

```bash
# 서버 실행 중인 터미널에서 로그 확인
# 또는 별도 터미널에서
tail -f logs/server.log  # 서버 로그가 파일로 저장되는 경우
```

## 🔧 테스트 스크립트 상세

### test_all.sh
- 모든 API 엔드포인트를 순차적으로 테스트
- 성공/실패 상태와 응답 미리보기 제공
- 각 테스트별로 개별 로그 파일 생성

### test_travel.sh
- 여행 일정 생성 API 집중 테스트
- 매개변수로 목적지와 기간 설정 가능
- 응답 데이터를 보기 좋게 파싱하여 출력

### test_health.sh
- 서버 상태 및 연결성 확인
- 데이터베이스, Gemma AI 상태 개별 확인
- 포트 접근성 및 HTTP 연결 테스트

### load_test.sh
- Apache Bench(ab)를 사용한 부하 테스트
- 헬스체크, 여행 생성, 계획 조회 API 부하 테스트
- 성능 분석 및 권장사항 제공

## 📋 테스트 시나리오

### 기본 시나리오
1. 헬스체크로 서버 상태 확인
2. 기본 여행 일정 생성 (필수값만)
3. 상세 옵션 포함 여행 일정 생성
4. 저장된 계획 목록 조회
5. 에러 케이스 테스트

### 고급 시나리오
1. 다양한 목적지 테스트 (부산, 제주도, 경주, 강릉 등)
2. 다양한 여행 유형 테스트 (가족, 혼자, 커플, 그룹)
3. 부하 테스트로 성능 확인
4. 에러 상황 테스트 (잘못된 데이터, 누락된 필드 등)

## 🐛 문제 해결

### 서버 연결 실패
```bash
# 서버가 실행 중인지 확인
curl http://localhost:8080/health

# 포트 사용 확인
lsof -i :8080
netstat -tulpn | grep 8080
```

### 환경 변수 확인
```bash
# 필수 환경 변수 확인
echo $GOOGLE_AI_API_KEY
echo $DB_HOST

# .env 파일 확인
cat ../../.env
```

### 권한 문제
```bash
# 스크립트 실행 권한 부여
chmod +x api/*.sh

# 로그 디렉토리 권한 확인
ls -la logs/
```

## 📚 API 엔드포인트 목록

| 엔드포인트 | 메소드 | 설명 |
|-----------|--------|------|
| `/health` | GET | 서버 상태 확인 |
| `/api/v1/travel/generate` | POST | 여행 일정 생성 |
| `/api/v1/travel/plans` | GET | 저장된 계획 목록 |
| `/api/v1/travel/plans/{id}` | GET | 특정 계획 상세 조회 |
| `/api/v1/travel/stats` | GET | 여행 통계 |
| `/api/v1/llm/generate` | POST | Gemma 모델 직접 테스트 |
| `/api/v1/llm/chat` | POST | Gemma 채팅 테스트 |

## 💡 팁

1. **jq 설치 권장**: JSON 응답을 보기 좋게 파싱하기 위해 `jq` 도구 설치
   ```bash
   # macOS
   brew install jq
   
   # Ubuntu
   sudo apt-get install jq
   ```

2. **로그 분석**: 테스트 후 로그 파일을 확인하여 상세한 요청/응답 분석

3. **성능 모니터링**: 부하 테스트 결과를 기반으로 서버 성능 최적화

4. **환경별 테스트**: 개발, 스테이징, 프로덕션 환경별로 BASE_URL 변경하여 테스트

## 🔗 관련 문서

- [TripWand Backend API 문서](../../CLAUDE.md)
- [프로젝트 README](../../README.md)
- [개발 가이드](../../docs/development.md)