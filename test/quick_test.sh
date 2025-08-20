#!/bin/bash

# TripWand Backend 빠른 테스트 스크립트
# 사용법: ./quick_test.sh

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}⚡ TripWand Backend 빠른 테스트${NC}"
echo "================================================"

# 1. 헬스체크
echo -e "${YELLOW}1. 헬스체크...${NC}"
if curl -s http://localhost:8080/health | jq -e '.status == "healthy"' >/dev/null 2>&1; then
    echo -e "${GREEN}✅ 서버 정상 동작${NC}"
else
    echo -e "${RED}❌ 서버 문제 발생${NC}"
    exit 1
fi

# 2. 간단한 여행 일정 생성 테스트
echo -e "${YELLOW}2. 여행 일정 생성 테스트...${NC}"
response=$(curl -s -X POST http://localhost:8080/api/v1/travel/generate \
  -H "Content-Type: application/json" \
  -d '{"destination": "서울", "duration": 2}')

if echo "$response" | jq -e '.success == true' >/dev/null 2>&1; then
    echo -e "${GREEN}✅ 여행 일정 생성 성공${NC}"
    destination=$(echo "$response" | jq -r '.meta.destination')
    duration=$(echo "$response" | jq -r '.meta.duration')
    cost=$(echo "$response" | jq -r '.data.estimated_cost')
    echo "   목적지: $destination, 기간: ${duration}일, 예상비용: ${cost}원"
else
    echo -e "${RED}❌ 여행 일정 생성 실패${NC}"
    echo "$response" | jq '.'
fi

# 3. 저장된 계획 조회 테스트
echo -e "${YELLOW}3. 저장된 계획 조회 테스트...${NC}"
plans_response=$(curl -s http://localhost:8080/api/v1/travel/plans?limit=5)

if echo "$plans_response" | jq -e '.success == true' >/dev/null 2>&1; then
    echo -e "${GREEN}✅ 계획 조회 성공${NC}"
    plan_count=$(echo "$plans_response" | jq -r '.data | length')
    total=$(echo "$plans_response" | jq -r '.meta.total')
    echo "   저장된 계획: ${plan_count}개 조회됨 (전체 ${total}개)"
else
    echo -e "${RED}❌ 계획 조회 실패${NC}"
fi

echo ""
echo -e "${BLUE}🎉 빠른 테스트 완료!${NC}"
echo ""
echo -e "${BLUE}💡 추가 테스트:${NC}"
echo "- 상세 테스트: cd test/api && ./test_all.sh"
echo "- 특정 목적지: cd test/api && ./test_travel.sh 부산 3"
echo "- 부하 테스트: cd test/api && ./load_test.sh"