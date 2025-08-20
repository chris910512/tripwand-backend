#!/bin/bash

# TripWand Backend 여행 API 테스트 스크립트
# 사용법: ./test_travel.sh [destination] [duration]

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 기본값
DEFAULT_DESTINATION="서울"
DEFAULT_DURATION=2
BASE_URL="http://localhost:8080"

# 매개변수 처리
DESTINATION=${1:-$DEFAULT_DESTINATION}
DURATION=${2:-$DEFAULT_DURATION}

echo -e "${BLUE}🎯 여행 일정 생성 테스트${NC}"
echo "================================================"
echo "목적지: $DESTINATION"
echo "기간: ${DURATION}일"
echo "서버: $BASE_URL"
echo ""

# 로그 디렉토리 생성
mkdir -p ../logs
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
LOG_FILE="../logs/${TIMESTAMP}_travel_${DESTINATION}_${DURATION}days.log"

# 요청 데이터 생성
REQUEST_DATA=$(cat <<EOF
{
    "destination": "$DESTINATION",
    "duration": $DURATION,
    "age_group": "20-30대",
    "group_size": 2,
    "purpose": "관광과 맛집 탐방",
    "travel_type": "균형잡힌 여행"
}
EOF
)

echo -e "${YELLOW}📋 요청 데이터:${NC}"
echo "$REQUEST_DATA" | jq '.'
echo ""

echo -e "${YELLOW}🚀 API 호출 중...${NC}"
echo ""

# API 호출 및 로깅
{
    echo "=== 여행 일정 생성 테스트 ==="
    echo "시간: $(date)"
    echo "목적지: $DESTINATION"
    echo "기간: ${DURATION}일"
    echo ""
    echo "요청 데이터:"
    echo "$REQUEST_DATA" | jq '.'
    echo ""
    echo "응답:"
} > "$LOG_FILE"

# curl 요청 실행
response=$(curl -s -w "\n%{http_code}" \
    -X POST "$BASE_URL/api/v1/travel/generate" \
    -H "Content-Type: application/json" \
    -d "$REQUEST_DATA")

# 응답 분리
http_code=$(echo "$response" | tail -1)
response_body=$(echo "$response" | sed '$d')

# 로그에 응답 기록
echo "HTTP 상태 코드: $http_code" >> "$LOG_FILE"
echo "$response_body" | jq '.' >> "$LOG_FILE" 2>/dev/null || echo "$response_body" >> "$LOG_FILE"

# 결과 출력
if [ "$http_code" -eq 200 ]; then
    echo -e "${GREEN}✅ 성공! (HTTP $http_code)${NC}"
    echo ""
    
    # 성공한 경우 응답 파싱 및 요약 출력
    if command -v jq >/dev/null 2>&1; then
        echo -e "${BLUE}📋 일정 요약:${NC}"
        echo "$response_body" | jq -r '
            if .success then
                "목적지: " + .meta.destination + 
                "\n기간: " + (.meta.duration | tostring) + "일" +
                "\n예상 비용: " + (.data.estimated_cost | tostring) + "원" +
                "\n\n📅 일차별 일정:" +
                (.data.itinerary[] | 
                    "\n\n🌅 " + (.day | tostring) + "일차:" +
                    "\n  아침: " + .morning.summary +
                    "\n  오후: " + .afternoon.summary + 
                    "\n  저녁: " + .evening.summary +
                    "\n  밤: " + .night.summary
                ) +
                "\n\n⚠️  주의사항:" +
                (.data.cautions[] | "\n  - " + .)
            else
                "❌ 오류: " + .message
            end
        '
    else
        echo "$response_body"
    fi
else
    echo -e "${RED}❌ 실패 (HTTP $http_code)${NC}"
    echo ""
    echo "오류 응답:"
    echo "$response_body" | jq '.' 2>/dev/null || echo "$response_body"
fi

echo ""
echo "상세 로그: $LOG_FILE"
echo ""
echo -e "${BLUE}💡 팁:${NC}"
echo "- 다른 목적지 테스트: ./test_travel.sh 제주도 4"
echo "- 전체 테스트 실행: ./test_all.sh"
echo "- 로그 확인: cat $LOG_FILE"