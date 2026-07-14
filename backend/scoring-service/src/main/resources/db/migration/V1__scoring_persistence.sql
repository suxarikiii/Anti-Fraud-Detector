CREATE TABLE IF NOT EXISTS scoring_calculations (
    dataset_id VARCHAR(100) NOT NULL,
    calculation_version BIGINT NOT NULL,
    feature_version BIGINT NOT NULL,
    feature_source VARCHAR(50) NOT NULL,
    calculated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (dataset_id, calculation_version)
);

CREATE TABLE IF NOT EXISTS scoring_results (
    dataset_id VARCHAR(100) NOT NULL,
    return_id VARCHAR(255) NOT NULL,
    calculation_version BIGINT NOT NULL,
    order_id VARCHAR(255) NOT NULL,
    customer_id VARCHAR(255) NOT NULL,
    support_agent_id VARCHAR(255) NOT NULL,
    order_amount DECIMAL(19, 4) NOT NULL,
    refund_amount DECIMAL(19, 4) NOT NULL,
    product_category VARCHAR(255) NOT NULL,
    return_reason VARCHAR(1000) NOT NULL,
    evidence_provided BOOLEAN NOT NULL,
    decision VARCHAR(50) NOT NULL,
    manual_override BOOLEAN NOT NULL,
    decision_time_minutes INTEGER NOT NULL,
    source_timestamp VARCHAR(100) NOT NULL,
    risk_score INTEGER NOT NULL,
    risk_level VARCHAR(20) NOT NULL,
    top_reason VARCHAR(2000) NOT NULL,
    feature_source VARCHAR(50) NOT NULL,
    relation_features TEXT NOT NULL,
    calculated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (dataset_id, return_id, calculation_version),
    FOREIGN KEY (dataset_id, calculation_version)
        REFERENCES scoring_calculations(dataset_id, calculation_version) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS risk_reasons (
    dataset_id VARCHAR(100) NOT NULL,
    return_id VARCHAR(255) NOT NULL,
    calculation_version BIGINT NOT NULL,
    reason_order INTEGER NOT NULL,
    reason_type VARCHAR(100) NOT NULL,
    message VARCHAR(2000) NOT NULL,
    score_impact INTEGER NOT NULL,
    PRIMARY KEY (dataset_id, return_id, calculation_version, reason_order),
    FOREIGN KEY (dataset_id, return_id, calculation_version)
        REFERENCES scoring_results(dataset_id, return_id, calculation_version) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS investigation_decisions (
    dataset_id VARCHAR(100) NOT NULL,
    return_id VARCHAR(255) NOT NULL,
    action VARCHAR(50) NOT NULL,
    outcome VARCHAR(50) NOT NULL,
    note TEXT NOT NULL,
    analyst_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    PRIMARY KEY (dataset_id, return_id)
);

CREATE TABLE IF NOT EXISTS scoring_processed_events (
    event_key VARCHAR(500) PRIMARY KEY,
    dataset_id VARCHAR(100) NOT NULL,
    job_id VARCHAR(100),
    feature_version BIGINT NOT NULL,
    scored_count INTEGER NOT NULL,
    suspicious_count INTEGER NOT NULL,
    processed_at TIMESTAMP WITH TIME ZONE NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scoring_results_dataset_risk
    ON scoring_results(dataset_id, calculation_version, risk_level);
CREATE INDEX IF NOT EXISTS idx_scoring_results_dataset_agent
    ON scoring_results(dataset_id, calculation_version, support_agent_id);
CREATE INDEX IF NOT EXISTS idx_risk_reasons_type
    ON risk_reasons(dataset_id, reason_type);
CREATE INDEX IF NOT EXISTS idx_decisions_outcome
    ON investigation_decisions(dataset_id, outcome);
