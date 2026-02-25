#!/bin/bash
# K8sWatch Disaster Recovery Drill Script
# This script executes a controlled DR drill to verify recovery procedures
# 
# Usage: ./dr-drill.sh [scenario]
# Scenarios: agent-failure, aggregator-failure, redis-failure, namespace-failure
#
# WARNING: This script will intentionally cause failures. Do not run in production without approval.

set -e

# Configuration
NAMESPACE="${NAMESPACE:-k8swatch}"
DRY_RUN="${DRY_RUN:-false}"
VERBOSE="${VERBOSE:-false}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check prerequisites
check_prerequisites() {
    log_info "Checking prerequisites..."
    
    if ! command -v kubectl &> /dev/null; then
        log_error "kubectl is required but not installed"
        exit 1
    fi
    
    if ! kubectl get namespace "$NAMESPACE" &> /dev/null; then
        log_error "Namespace $NAMESPACE does not exist"
        exit 1
    fi
    
    log_success "Prerequisites check passed"
}

# Record start time
record_start_time() {
    START_TIME=$(date +%s)
    echo "$START_TIME" > /tmp/dr-drill-start-time
    log_info "DR Drill started at $(date)"
}

# Record end time and calculate duration
record_end_time() {
    END_TIME=$(date +%s)
    START_TIME=$(cat /tmp/dr-drill-start-time 2>/dev/null || echo "$END_TIME")
    DURATION=$((END_TIME - START_TIME))
    echo "$DURATION" > /tmp/dr-drill-duration
    log_info "DR Drill completed in ${DURATION}s"
}

# Verify recovery
verify_recovery() {
    local component=$1
    local expected_replicas=$2
    
    log_info "Verifying $component recovery..."
    
    # Wait for pods to be ready
    if ! kubectl wait --for=condition=ready pod \
        -l "app.kubernetes.io/component=$component" \
        -n "$NAMESPACE" \
        --timeout=300s &> /dev/null; then
        log_error "$component recovery verification failed"
        return 1
    fi
    
    # Check replica count
    local actual_replicas
    actual_replicas=$(kubectl get pods -n "$NAMESPACE" -l "app.kubernetes.io/component=$component" --field-selector=status.phase=Running --no-headers | wc -l)
    
    if [ "$actual_replicas" -lt "$expected_replicas" ]; then
        log_error "$component has $actual_replicas replicas, expected $expected_replicas"
        return 1
    fi
    
    log_success "$component recovered successfully ($actual_replicas replicas)"
    return 0
}

# Scenario 1: Agent Pod Failure
drill_agent_failure() {
    log_info "=== Scenario 1: Agent Pod Failure ==="
    log_warning "This will delete agent pods one at a time"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY RUN] Would delete agent pods"
        return 0
    fi
    
    # Get agent pods
    local agent_pods
    agent_pods=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=agent -o jsonpath='{.items[*].metadata.name}')
    
    for pod in $agent_pods; do
        log_info "Deleting agent pod: $pod"
        kubectl delete pod -n "$NAMESPACE" "$pod" --grace-period=30
        
        # Wait for pod to be ready
        if ! kubectl wait --for=condition=ready pod -n "$NAMESPACE" "$pod" --timeout=120s &> /dev/null; then
            log_error "Agent pod $pod failed to recover"
            return 1
        fi
        
        log_success "Agent pod $pod recovered"
        sleep 5
    done
    
    log_success "Agent failure drill completed successfully"
    return 0
}

# Scenario 2: Aggregator Pod Failure
drill_aggregator_failure() {
    log_info "=== Scenario 2: Aggregator Pod Failure ==="
    log_warning "This will delete all aggregator pods simultaneously"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY RUN] Would delete aggregator pods"
        return 0
    fi
    
    # Record start time
    local start_time
    start_time=$(date +%s)
    
    # Delete all aggregator pods
    kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=aggregator
    
    # Wait for recovery
    if ! verify_recovery "aggregator" 3; then
        log_error "Aggregator failure drill failed"
        return 1
    fi
    
    # Calculate recovery time
    local end_time
    end_time=$(date +%s)
    local recovery_time=$((end_time - start_time))
    
    log_success "Aggregator recovered in ${recovery_time}s (RTO target: 120s)"
    
    if [ "$recovery_time" -gt 120 ]; then
        log_warning "Recovery time exceeded RTO target"
    fi
    
    log_success "Aggregator failure drill completed successfully"
    return 0
}

# Scenario 3: Redis Failure
drill_redis_failure() {
    log_info "=== Scenario 3: Redis Failure ==="
    log_warning "This will delete the Redis pod"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY RUN] Would delete Redis pod"
        return 0
    fi
    
    # Record start time
    local start_time
    start_time=$(date +%s)
    
    # Delete Redis pod
    kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/name=redis
    
    # Wait for recovery
    if ! verify_recovery "state-store" 1; then
        log_error "Redis failure drill failed"
        return 1
    fi
    
    # Calculate recovery time
    local end_time
    end_time=$(date +%s)
    local recovery_time=$((end_time - start_time))
    
    log_success "Redis recovered in ${recovery_time}s (RTO target: 120s)"
    
    # Verify aggregator can connect to Redis
    log_info "Verifying aggregator connectivity to Redis..."
    local aggregator_pod
    aggregator_pod=$(kubectl get pods -n "$NAMESPACE" -l app.kubernetes.io/component=aggregator -o jsonpath='{.items[0].metadata.name}')
    
    if ! kubectl exec -n "$NAMESPACE" "$aggregator_pod" -- redis-cli -h k8swatch-redis ping &> /dev/null; then
        log_error "Aggregator cannot connect to Redis after recovery"
        return 1
    fi
    
    log_success "Redis connectivity verified"
    log_success "Redis failure drill completed successfully"
    return 0
}

# Scenario 4: AlertManager Failure
drill_alertmanager_failure() {
    log_info "=== Scenario 4: AlertManager Failure ==="
    log_warning "This will delete the AlertManager pod"
    
    if [ "$DRY_RUN" = "true" ]; then
        log_info "[DRY RUN] Would delete AlertManager pod"
        return 0
    fi
    
    # Record start time
    local start_time
    start_time=$(date +%s)
    
    # Delete AlertManager pod
    kubectl delete pods -n "$NAMESPACE" -l app.kubernetes.io/component=alertmanager
    
    # Wait for recovery
    if ! verify_recovery "alertmanager" 2; then
        log_error "AlertManager failure drill failed"
        return 1
    fi
    
    # Calculate recovery time
    local end_time
    end_time=$(date +%s)
    local recovery_time=$((end_time - start_time))
    
    log_success "AlertManager recovered in ${recovery_time}s (RTO target: 120s)"
    log_success "AlertManager failure drill completed successfully"
    return 0
}

# Full DR Drill (all scenarios)
drill_full() {
    log_info "=== Full DR Drill ==="
    log_warning "This will execute all DR scenarios sequentially"
    
    local failed=0
    
    drill_agent_failure || ((failed++))
    sleep 30
    
    drill_aggregator_failure || ((failed++))
    sleep 30
    
    drill_redis_failure || ((failed++))
    sleep 30
    
    drill_alertmanager_failure || ((failed++))
    
    if [ "$failed" -gt 0 ]; then
        log_error "$failed scenario(s) failed"
        return 1
    fi
    
    log_success "All DR scenarios completed successfully"
    return 0
}

# Generate drill report
generate_report() {
    local scenario=$1
    local status=$2
    local duration=$3
    
    local report_file="/tmp/dr-drill-report-$(date +%Y%m%d-%H%M%S).md"
    
    cat > "$report_file" << EOF
# DR Drill Report

**Date:** $(date)
**Scenario:** $scenario
**Status:** $status
**Duration:** ${duration}s

## Recovery Time Objectives

| Component | RTO Target | Actual | Status |
|-----------|------------|--------|--------|
| Agent | 60s | TBD | $status |
| Aggregator | 120s | TBD | $status |
| Redis | 120s | TBD | $status |
| AlertManager | 120s | TBD | $status |

## Observations

- TODO: Add observations

## Action Items

- [ ] Review recovery times
- [ ] Update procedures if needed
- [ ] Schedule next drill

## Sign-off

- [ ] SRE Lead
- [ ] Platform Team

---
Report generated: $report_file
EOF

    log_info "Drill report saved to: $report_file"
}

# Main function
main() {
    local scenario="${1:-full}"
    
    echo "========================================"
    echo "K8sWatch Disaster Recovery Drill"
    echo "========================================"
    echo ""
    
    check_prerequisites
    
    record_start_time
    
    case "$scenario" in
        agent-failure)
            drill_agent_failure
            ;;
        aggregator-failure)
            drill_aggregator_failure
            ;;
        redis-failure)
            drill_redis_failure
            ;;
        alertmanager-failure)
            drill_alertmanager_failure
            ;;
        full)
            drill_full
            ;;
        *)
            log_error "Unknown scenario: $scenario"
            echo "Valid scenarios: agent-failure, aggregator-failure, redis-failure, alertmanager-failure, full"
            exit 1
            ;;
    esac
    
    local exit_code=$?
    
    record_end_time
    
    local duration
    duration=$(cat /tmp/dr-drill-duration 2>/dev/null || echo "0")
    
    if [ "$exit_code" -eq 0 ]; then
        generate_report "$scenario" "SUCCESS" "$duration"
        log_success "DR drill completed successfully in ${duration}s"
    else
        generate_report "$scenario" "FAILED" "$duration"
        log_error "DR drill failed"
    fi
    
    exit $exit_code
}

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --verbose)
            VERBOSE=true
            shift
            ;;
        --namespace)
            NAMESPACE="$2"
            shift 2
            ;;
        *)
            SCENARIO="$1"
            shift
            ;;
    esac
done

main "${SCENARIO:-full}"
