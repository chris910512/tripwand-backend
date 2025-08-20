#!/bin/bash

# TripWand Backend 헬스체크 테스트 스크립트
# 사용법: ./test_health.sh

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

BASE_URL="http://localhost:8080"

echo -e "${BLUE}🩺 헬스체크 테스트${NC}"
echo "================================================"
echo "서버: $BASE_URL"
echo "시간: $(date)"
echo ""

# 로그 파일 생성
mkdir -p ../logs
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
LOG_FILE="../logs/${TIMESTAMP}_health_check.log"

echo -e "${YELLOW}🚀 헬스체크 API 호출 중...${NC}"

# API 호출
response=$(curl -s -w "\n%{http_code}" "$BASE_URL/health")
http_code=$(echo "$response" | tail -1)
response_body=$(echo "$response" | sed '$d')

# 로그 기록
{
    echo "=== 헬스체크 테스트 ==="
    echo "시간: $(date)"
    echo "URL: $BASE_URL/health"
    echo ""
    echo "HTTP 상태 코드: $http_code"
    echo "응답 내용:"
    echo "$response_body" | jq '.' 2>/dev/null || echo "$response_body"
} > "$LOG_FILE"

# 결과 출력
if [ "$http_code" -eq 200 ]; then
    echo -e "${GREEN}✅ 서버 정상 동작 (HTTP $http_code)${NC}"
    echo ""
    
    if command -v jq >/dev/null 2>&1; then
        echo -e "${BLUE}📊 서비스 상태:${NC}"
        echo "$response_body" | jq -r '
            "서비스: " + .service + 
            "\n버전: " + .version +
            "\n상태: " + .status +
            "\n데이터베이스: " + .database +
            "\nGemma AI: " + .gemma +
            "\n\n🛠️  사용 가능한 기능:" +
            (.features[] | "\n  - " + .) +
            "\n\n🔗 API 엔드포인트:" +
            (.endpoints[] | "\n  - " + .)
        '
    else
        echo "$response_body"
    fi
    
    # 개별 서비스 상태 확인
    if echo "$response_body" | jq -e '.database == "healthy"' >/dev/null 2>&1; then
        echo -e "\n${GREEN}🗄️  데이터베이스: 정상${NC}"
    else
        echo -e "\n${RED}🗄️  데이터베이스: 문제 발생${NC}"
    fi
    
    if echo "$response_body" | jq -e '.gemma == "healthy"' >/dev/null 2>&1; then
        echo -e "${GREEN}🤖 Gemma AI: 정상${NC}"
    else
        echo -e "${RED}🤖 Gemma AI: 문제 발생${NC}"
    fi
    
else
    echo -e "${RED}❌ 서버 문제 발생 (HTTP $http_code)${NC}"
    echo ""
    echo "오류 응답:"
    echo "$response_body"
    
    # 서버가 실행 중인지 확인
    if ! curl -s "$BASE_URL/health" >/dev/null 2>&1; then
        echo ""
        echo -e "${YELLOW}💡 서버가 실행되지 않은 것 같습니다.${NC}"
        echo "서버 시작: go run cmd/main.go"
    fi
fi

echo ""
echo "상세 로그: $LOG_FILE"
echo ""

# 추가 연결성 테스트
echo -e "${BLUE}🔍 추가 연결성 테스트${NC}"
echo "================================================"

# 포트 8080이 열려있는지 확인
if nc -z localhost 8080 2>/dev/null; then
    echo -e "${GREEN}✅ 포트 8080 접근 가능${NC}"
else
    echo -e "${RED}❌ 포트 8080 접근 불가${NC}"
fi

# 기본 HTTP 연결 테스트
if curl -s --connect-timeout 5 "$BASE_URL" >/dev/null 2>&1; then
    echo -e "${GREEN}✅ HTTP 연결 성공${NC}"
else
    echo -e "${RED}❌ HTTP 연결 실패${NC}"
fi

echo ""
echo -e "${BLUE}💡 다음 단계:${NC}"
echo "- 여행 API 테스트: ./test_travel.sh"
echo "- 전체 테스트: ./test_all.sh"
echo "- 개발 서버 시작: cd ../.. && go run cmd/main.go"