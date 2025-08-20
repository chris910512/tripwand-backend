#!/bin/bash

# TripWand Backend 부하 테스트 스크립트
# 사용법: ./load_test.sh [동시요청수] [총요청수]

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 기본값
DEFAULT_CONCURRENT=5
DEFAULT_TOTAL=20
BASE_URL="http://localhost:8080"

# 매개변수 처리
CONCURRENT=${1:-$DEFAULT_CONCURRENT}
TOTAL=${2:-$DEFAULT_TOTAL}

echo -e "${BLUE}⚡ 부하 테스트${NC}"
echo "================================================"
echo "동시 요청수: $CONCURRENT"
echo "총 요청수: $TOTAL"
echo "서버: $BASE_URL"
echo "시간: $(date)"
echo ""

# 필수 도구 확인
if ! command -v ab >/dev/null 2>&1; then
    echo -e "${RED}❌ Apache Bench (ab) 가 설치되지 않았습니다.${NC}"
    echo ""
    echo "설치 방법:"
    echo "  macOS: brew install httpd"
    echo "  Ubuntu: sudo apt-get install apache2-utils"
    echo "  CentOS: sudo yum install httpd-tools"
    exit 1
fi

# 로그 디렉토리 생성
mkdir -p ../logs ../data
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')
LOG_FILE="../logs/${TIMESTAMP}_load_test.log"

# 테스트 데이터 파일 생성
TEST_DATA_FILE="../data/test_request.json"
cat > "$TEST_DATA_FILE" << EOF
{
    "destination": "서울",
    "duration": 2,
    "age_group": "20-30대",
    "group_size": 2,
    "purpose": "관광과 맛집",
    "travel_type": "균형잡힌 여행"
}
EOF

echo -e "${YELLOW}📋 테스트 데이터:${NC}"
cat "$TEST_DATA_FILE" | jq '.'
echo ""

# 1. 헬스체크 부하 테스트
echo -e "${YELLOW}🩺 헬스체크 부하 테스트${NC}"
echo "================================================"

{
    echo "=== TripWand Backend 부하 테스트 ==="
    echo "시간: $(date)"
    echo "동시 요청수: $CONCURRENT"
    echo "총 요청수: $TOTAL"
    echo ""
    echo "=== 1. 헬스체크 부하 테스트 ==="
} > "$LOG_FILE"

echo "헬스체크 API 부하 테스트 실행 중..."
ab -n "$TOTAL" -c "$CONCURRENT" "$BASE_URL/health" >> "$LOG_FILE" 2>&1

echo -e "${GREEN}✅ 헬스체크 부하 테스트 완료${NC}"
echo ""

# 2. 여행 일정 생성 부하 테스트
echo -e "${YELLOW}🎯 여행 일정 생성 부하 테스트${NC}"
echo "================================================"

echo "" >> "$LOG_FILE"
echo "=== 2. 여행 일정 생성 부하 테스트 ===" >> "$LOG_FILE"

echo "여행 일정 생성 API 부하 테스트 실행 중..."
echo "⚠️  AI 처리로 인해 시간이 오래 걸릴 수 있습니다..."

ab -n "$TOTAL" -c "$CONCURRENT" \
   -T "application/json" \
   -p "$TEST_DATA_FILE" \
   "$BASE_URL/api/v1/travel/generate" >> "$LOG_FILE" 2>&1

echo -e "${GREEN}✅ 여행 일정 생성 부하 테스트 완료${NC}"
echo ""

# 3. 저장된 계획 조회 부하 테스트
echo -e "${YELLOW}📚 저장된 계획 조회 부하 테스트${NC}"
echo "================================================"

echo "" >> "$LOG_FILE"
echo "=== 3. 저장된 계획 조회 부하 테스트 ===" >> "$LOG_FILE"

echo "저장된 계획 조회 API 부하 테스트 실행 중..."
ab -n "$TOTAL" -c "$CONCURRENT" \
   "$BASE_URL/api/v1/travel/plans?page=1&limit=10" >> "$LOG_FILE" 2>&1

echo -e "${GREEN}✅ 저장된 계획 조회 부하 테스트 완료${NC}"
echo ""

# 결과 요약 출력
echo -e "${BLUE}📊 테스트 결과 요약${NC}"
echo "================================================"

# 헬스체크 결과 요약
echo -e "${YELLOW}🩺 헬스체크:${NC}"
grep "Requests per second" "$LOG_FILE" | head -1 | sed 's/^/  /'
grep "Time per request" "$LOG_FILE" | head -2 | sed 's/^/  /'

echo ""

# 여행 일정 생성 결과 요약
echo -e "${YELLOW}🎯 여행 일정 생성:${NC}"
grep "Requests per second" "$LOG_FILE" | sed -n '2p' | sed 's/^/  /'
grep "Time per request" "$LOG_FILE" | sed -n '3,4p' | sed 's/^/  /'

echo ""

# 저장된 계획 조회 결과 요약
echo -e "${YELLOW}📚 저장된 계획 조회:${NC}"
grep "Requests per second" "$LOG_FILE" | tail -1 | sed 's/^/  /'
grep "Time per request" "$LOG_FILE" | tail -2 | sed 's/^/  /'

echo ""

# 실패한 요청 확인
failed_requests=$(grep "Failed requests" "$LOG_FILE" | awk '{sum += $3} END {print sum+0}')
if [ "$failed_requests" -gt 0 ]; then
    echo -e "${RED}⚠️  실패한 요청: $failed_requests${NC}"
else
    echo -e "${GREEN}✅ 모든 요청 성공${NC}"
fi

echo ""
echo "상세 결과: $LOG_FILE"
echo ""

# 성능 분석 및 권장사항
echo -e "${BLUE}💡 성능 분석 및 권장사항${NC}"
echo "================================================"

# 평균 응답 시간 확인
avg_time=$(grep "Time per request" "$LOG_FILE" | head -1 | awk '{print $4}')
if (( $(echo "$avg_time > 1000" | bc -l) )); then
    echo -e "${RED}⚠️  평균 응답시간이 1초를 초과합니다 (${avg_time}ms)${NC}"
    echo "   권장사항:"
    echo "   - 데이터베이스 연결 풀 크기 조정"
    echo "   - Gemma AI 호출 최적화"
    echo "   - 캐싱 레이어 추가 고려"
elif (( $(echo "$avg_time > 500" | bc -l) )); then
    echo -e "${YELLOW}⚠️  평균 응답시간이 다소 높습니다 (${avg_time}ms)${NC}"
    echo "   권장사항:"
    echo "   - 응답 캐싱 고려"
    echo "   - 데이터베이스 쿼리 최적화"
else
    echo -e "${GREEN}✅ 양호한 응답시간입니다 (${avg_time}ms)${NC}"
fi

echo ""
echo -e "${BLUE}🔧 추가 테스트 옵션:${NC}"
echo "- 더 높은 부하: ./load_test.sh 10 50"
echo "- 장시간 테스트: ./load_test.sh 3 100"
echo "- 개별 API 테스트: ./test_travel.sh"