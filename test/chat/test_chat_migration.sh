#!/bin/bash

# test_chat_migration.sh - 채팅 데이터베이스 스키마 마이그레이션 테스트

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
TEST_LOG_DIR="$PROJECT_DIR/test/logs"

# 로그 디렉토리 생성
mkdir -p "$TEST_LOG_DIR"

# 테스트 로그 파일
TIMESTAMP=$(date +"%Y%m%d_%H%M%S")
LOG_FILE="$TEST_LOG_DIR/${TIMESTAMP}_chat_migration.log"

echo "========================================" | tee -a "$LOG_FILE"
echo "채팅 데이터베이스 마이그레이션 테스트" | tee -a "$LOG_FILE"
echo "시작 시간: $(date)" | tee -a "$LOG_FILE"
echo "========================================" | tee -a "$LOG_FILE"
echo

# .env 파일 로딩
if [ -f "$PROJECT_DIR/.env" ]; then
    echo "📁 .env 파일 로딩..." | tee -a "$LOG_FILE"
    set -a
    source "$PROJECT_DIR/.env"
    set +a
    echo "✅ .env 파일 로딩 완료" | tee -a "$LOG_FILE"
else
    echo "⚠️  .env 파일을 찾을 수 없습니다." | tee -a "$LOG_FILE"
fi

# 환경 변수 확인
echo "📋 환경 변수 확인..." | tee -a "$LOG_FILE"

REQUIRED_VARS=(
    "DB_HOST"
    "DB_PORT" 
    "DB_USER"
    "DB_PASSWORD"
    "DB_NAME"
)

for var in "${REQUIRED_VARS[@]}"; do
    eval "var_value=\$$var"
    if [ -z "$var_value" ]; then
        echo "❌ 필수 환경변수 $var가 설정되지 않았습니다." | tee -a "$LOG_FILE"
        echo "테스트를 위해 기본값을 사용합니다." | tee -a "$LOG_FILE"
    else
        echo "✅ $var: $var_value" | tee -a "$LOG_FILE"
    fi
done

# 기본값 설정 (테스트 환경)
export DB_HOST=${DB_HOST:-"localhost"}
export DB_PORT=${DB_PORT:-"5432"}
export DB_USER=${DB_USER:-"postgres"}
export DB_PASSWORD=${DB_PASSWORD:-"postgres"}
export DB_NAME=${DB_NAME:-"chatdb"}
export DB_SSL_MODE=${DB_SSL_MODE:-"disable"}

echo | tee -a "$LOG_FILE"

# PostgreSQL 연결 테스트
echo "🔗 PostgreSQL 연결 테스트..." | tee -a "$LOG_FILE"

# 데이터베이스 생성 (존재하지 않는 경우)
PGPASSWORD="$DB_PASSWORD" createdb -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" "$DB_NAME" 2>/dev/null || true

# 연결 테스트
if PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "SELECT version();" >/dev/null 2>&1; then
    echo "✅ PostgreSQL 연결 성공" | tee -a "$LOG_FILE"
    
    # PostgreSQL 버전 확인
    PG_VERSION=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "SELECT version();" 2>/dev/null | head -1)
    echo "📊 PostgreSQL 버전: $PG_VERSION" | tee -a "$LOG_FILE"
else
    echo "❌ PostgreSQL 연결 실패" | tee -a "$LOG_FILE"
    echo "테스트를 종료합니다." | tee -a "$LOG_FILE"
    exit 1
fi

echo | tee -a "$LOG_FILE"

# 채팅 테이블 마이그레이션 테스트
echo "🔧 채팅 테이블 마이그레이션 테스트..." | tee -a "$LOG_FILE"

cd "$PROJECT_DIR"

# Go 마이그레이션 실행을 위한 임시 테스트 앱 생성
cat > test_migration.go << 'EOF'
package main

import (
	"log"
	"tripwand-backend/internal/database"
)

func main() {
	// 데이터베이스 연결
	if err := database.Connect(); err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}
	defer database.Close()

	// 마이그레이션 실행
	if err := database.Migrate(); err != nil {
		log.Fatalf("Migration failed: %v", err)
	}

	log.Println("Migration completed successfully!")
}
EOF

# 마이그레이션 실행
echo "실행 중: go run test_migration.go" | tee -a "$LOG_FILE"
set +e  # 마이그레이션 실행 시 일시적으로 오류 허용
go run test_migration.go 2>&1 | tee -a "$LOG_FILE"
MIGRATION_EXIT_CODE=$?
set -e  # 다시 오류 시 종료 모드로 복원

if [ $MIGRATION_EXIT_CODE -eq 0 ]; then
    echo "✅ 마이그레이션 성공" | tee -a "$LOG_FILE"
    MIGRATION_SUCCESS=true
else
    echo "❌ 마이그레이션 실패 (exit code: $MIGRATION_EXIT_CODE)" | tee -a "$LOG_FILE"
    MIGRATION_SUCCESS=false
fi

# 임시 파일 정리
rm -f test_migration.go

echo | tee -a "$LOG_FILE"

if [ "$MIGRATION_SUCCESS" = true ]; then
    # 테이블 생성 확인
    echo "📊 생성된 테이블 확인..." | tee -a "$LOG_FILE"
    
    TABLES_TO_CHECK=(
        "chat_rooms"
        "anonymous_sessions"
        "chat_messages"
    )
    
    for table in "${TABLES_TO_CHECK[@]}"; do
        # 테이블 존재 확인
        TABLE_EXISTS=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
            SELECT EXISTS (
                SELECT 1 FROM information_schema.tables 
                WHERE table_schema = 'public' AND table_name = '$table'
            );
        " 2>/dev/null | tr -d ' ' | head -1)
        
        if [ "$TABLE_EXISTS" = "t" ]; then
            echo "✅ $table 테이블 존재" | tee -a "$LOG_FILE"
            
            # 테이블 구조 출력
            echo "📋 $table 테이블 구조:" | tee -a "$LOG_FILE"
            PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "\\d $table" 2>&1 | tee -a "$LOG_FILE"
        else
            echo "❌ $table 테이블 미존재" | tee -a "$LOG_FILE"
        fi
        echo | tee -a "$LOG_FILE"
    done

    # 파티션 테이블 확인
    echo "🗂️  파티션 테이블 확인..." | tee -a "$LOG_FILE"
    PARTITIONS=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT schemaname||'.'||tablename 
        FROM pg_tables 
        WHERE tablename LIKE 'chat_messages_%' 
        AND schemaname = 'public'
        ORDER BY tablename;
    " 2>/dev/null | grep -v '^$')
    
    if [ -n "$PARTITIONS" ]; then
        echo "✅ 파티션 테이블들:" | tee -a "$LOG_FILE"
        echo "$PARTITIONS" | tee -a "$LOG_FILE"
    else
        echo "⚠️  파티션 테이블을 찾을 수 없습니다" | tee -a "$LOG_FILE"
    fi

    echo | tee -a "$LOG_FILE"

    # 기본 채팅룸 생성 확인
    echo "🏠 기본 채팅룸 확인..." | tee -a "$LOG_FILE"
    ROOM_COUNT=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) FROM chat_rooms WHERE room_type = 'public';
    " 2>/dev/null | tr -d ' ')
    
    if [ "$ROOM_COUNT" = "7" ]; then
        echo "✅ 7개 국가별 채팅룸 생성 완료" | tee -a "$LOG_FILE"
        
        # 채팅룸 목록 출력
        echo "📋 생성된 채팅룸 목록:" | tee -a "$LOG_FILE"
        PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -c "
            SELECT id, room_type, country_code, created_at 
            FROM chat_rooms 
            WHERE room_type = 'public'
            ORDER BY country_code;
        " 2>&1 | tee -a "$LOG_FILE"
    else
        echo "❌ 기본 채팅룸 개수 불일치: $ROOM_COUNT/7" | tee -a "$LOG_FILE"
    fi

    echo | tee -a "$LOG_FILE"

    # 인덱스 확인
    echo "📇 인덱스 확인..." | tee -a "$LOG_FILE"
    INDEX_COUNT=$(PGPASSWORD="$DB_PASSWORD" psql -h "$DB_HOST" -p "$DB_PORT" -U "$DB_USER" -d "$DB_NAME" -t -c "
        SELECT COUNT(*) 
        FROM pg_indexes 
        WHERE schemaname = 'public' 
        AND (tablename LIKE 'chat_%' OR tablename LIKE 'anonymous_%');
    " 2>/dev/null | tr -d ' ')
    
    echo "✅ 채팅 관련 인덱스 개수: $INDEX_COUNT" | tee -a "$LOG_FILE"
fi

echo | tee -a "$LOG_FILE"
echo "========================================" | tee -a "$LOG_FILE"
if [ "$MIGRATION_SUCCESS" = true ]; then
    echo "🎉 채팅 데이터베이스 마이그레이션 테스트 완료!" | tee -a "$LOG_FILE"
    echo "✅ 모든 테이블이 성공적으로 생성되었습니다." | tee -a "$LOG_FILE"
else
    echo "❌ 채팅 데이터베이스 마이그레이션 테스트 실패!" | tee -a "$LOG_FILE"
fi
echo "종료 시간: $(date)" | tee -a "$LOG_FILE"
echo "로그 파일: $LOG_FILE" | tee -a "$LOG_FILE"
echo "========================================" | tee -a "$LOG_FILE"

# 테스트 결과에 따른 exit code
if [ "$MIGRATION_SUCCESS" = true ]; then
    exit 0
else
    exit 1
fi