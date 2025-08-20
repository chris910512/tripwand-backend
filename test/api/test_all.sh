#!/bin/bash

# TripWand Backend API 전체 테스트 스크립트
# 사용법: ./test_all.sh

set -e

# 색상 정의
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 서버 URL
BASE_URL="http://localhost:8080"

# 로그 디렉토리 생성
mkdir -p ../logs
LOG_DIR="../logs"
TIMESTAMP=$(date '+%Y%m%d_%H%M%S')

echo -e "${BLUE}🚀 TripWand Backend API 테스트 시작${NC}"
echo "================================================"
echo "타임스탬프: $TIMESTAMP"
echo "서버 URL: $BASE_URL"
echo "로그 디렉토리: $LOG_DIR"
echo ""

# 헬퍼 함수: API 요청 로깅
log_request() {
    local test_name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local log_file="$LOG_DIR/${TIMESTAMP}_${test_name}.log"
    
    echo -e "${YELLOW}📋 테스트: $test_name${NC}"
    echo "요청: $method $url"
    if [ ! -z "$data" ]; then
        echo "데이터: $data"
    fi
    echo ""
    
    # 요청 로그 기록
    echo "=== $test_name ===" >> "$log_file"
    echo "시간: $(date)" >> "$log_file"
    echo "요청: $method $url" >> "$log_file"
    if [ ! -z "$data" ]; then
        echo "요청 데이터:" >> "$log_file"
        echo "$data" | jq '.' >> "$log_file" 2>/dev/null || echo "$data" >> "$log_file"
    fi
    echo "" >> "$log_file"
    
    # API 호출 및 응답 로깅
    if [ -z "$data" ]; then
        response=$(curl -s -w "\n%{http_code}" "$url")
    else
        response=$(curl -s -w "\n%{http_code}" -X "$method" -H "Content-Type: application/json" -d "$data" "$url")
    fi
    
    http_code=$(echo "$response" | tail -1)
    response_body=$(echo "$response" | sed '$d')
    
    echo "응답 코드: $http_code" >> "$log_file"
    echo "응답 내용:" >> "$log_file"
    echo "$response_body" | jq '.' >> "$log_file" 2>/dev/null || echo "$response_body" >> "$log_file"
    echo "" >> "$log_file"
    echo "================================================" >> "$log_file"
    echo "" >> "$log_file"
    
    # 결과 출력
    if [ "$http_code" -eq 200 ]; then
        echo -e "${GREEN}✅ 성공 (HTTP $http_code)${NC}"
    else
        echo -e "${RED}❌ 실패 (HTTP $http_code)${NC}"
    fi
    
    # 응답 미리보기 (첫 200자)
    preview=$(echo "$response_body" | head -c 200)
    echo "응답 미리보기: $preview..."
    echo ""
    echo "상세 로그: $log_file"
    echo ""
}

# 1. 헬스체크
log_request "health_check" "GET" "$BASE_URL/health"

# 2. 기본 여행 일정 생성
busan_basic_data='{
    "destination": "부산",
    "duration": 3
}'
log_request "busan_basic" "POST" "$BASE_URL/api/v1/travel/generate" "$busan_basic_data"

# 3. 상세 옵션 포함 여행 일정
jeju_detailed_data='{
    "destination": "제주도",
    "duration": 4,
    "age_group": "20대",
    "group_size": 2,
    "purpose": "힐링과 휴식",
    "travel_type": "여유로운 여행"
}'
log_request "jeju_detailed" "POST" "$BASE_URL/api/v1/travel/generate" "$jeju_detailed_data"

# 4. 가족 여행 일정
gyeongju_family_data='{
    "destination": "경주",
    "duration": 2,
    "age_group": "30-40대",
    "group_size": 4,
    "purpose": "문화 체험과 교육",
    "travel_type": "교육적인 여행"
}'
log_request "gyeongju_family" "POST" "$BASE_URL/api/v1/travel/generate" "$gyeongju_family_data"

# 5. 혼자 하는 배낭여행
gangneung_solo_data='{
    "destination": "강릉",
    "duration": 5,
    "age_group": "20대",
    "group_size": 1,
    "purpose": "자연 탐방과 사색",
    "travel_type": "자유로운 배낭여행"
}'
log_request "gangneung_solo" "POST" "$BASE_URL/api/v1/travel/generate" "$gangneung_solo_data"

# 6. 저장된 여행 계획 목록 조회
log_request "plans_list" "GET" "$BASE_URL/api/v1/travel/plans?page=1&limit=10"

# 7. 부산 계획 필터링
log_request "plans_filter_busan" "GET" "$BASE_URL/api/v1/travel/plans?destination=부산"

# 8. 여행 통계 조회
log_request "travel_stats" "GET" "$BASE_URL/api/v1/travel/stats"

# 9. Gemma 모델 직접 테스트
gemma_test_data='{
    "prompt": "안녕하세요! 테스트 메시지입니다.",
    "temperature": 0.7,
    "max_tokens": 100
}'
log_request "gemma_direct" "POST" "$BASE_URL/api/v1/llm/generate" "$gemma_test_data"

# 10. 에러 테스트 - 필수값 누락
error_missing_data='{
    "duration": 3
}'
log_request "error_missing_destination" "POST" "$BASE_URL/api/v1/travel/generate" "$error_missing_data"

# 11. 에러 테스트 - 잘못된 기간
error_invalid_duration_data='{
    "destination": "서울",
    "duration": 50
}'
log_request "error_invalid_duration" "POST" "$BASE_URL/api/v1/travel/generate" "$error_invalid_duration_data"

echo ""
echo -e "${BLUE}🏁 모든 테스트 완료!${NC}"
echo "================================================"
echo "로그 파일들이 $LOG_DIR 디렉토리에 저장되었습니다."
echo ""
echo "로그 파일 목록:"
ls -la "$LOG_DIR"/${TIMESTAMP}_*.log
echo ""
echo "전체 로그 확인: cat $LOG_DIR/${TIMESTAMP}_*.log"
echo "특정 테스트 로그: cat $LOG_DIR/${TIMESTAMP}_[테스트명].log"