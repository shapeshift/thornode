CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

CREATE TABLE block_log (
	height			BIGINT NOT NULL,
	timestamp		BIGINT NOT NULL,
	hash			BYTEA NOT NULL,
	agg_state		BYTEA,
	PRIMARY KEY (height)
);

-- Sparse table for depths.
-- Only those height/pool pairs are filled where there is a change.
-- For missing values, use the latest existing height for a pool.
-- Asset and Rune are filled together, it's not needed to look back for them separately.
CREATE TABLE block_pool_depths (
	pool				VARCHAR(60) NOT NULL,
	asset_E8			BIGINT NOT NULL,
	rune_E8				BIGINT NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('block_pool_depths', 'block_timestamp', chunk_time_interval => 86400000000000);
CREATE INDEX ON block_pool_depths (pool, block_timestamp DESC);

CREATE TABLE active_vault_events (
	add_asgard_addr		VARCHAR(90) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('active_vault_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE add_events (
	tx  			VARCHAR(64) NOT NULL,
	chain			VARCHAR(8) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	asset			VARCHAR(60),
	asset_E8		BIGINT NOT NULL,
	memo			TEXT NOT NULL,
	rune_E8			BIGINT NOT NULL,
	pool			VARCHAR(60) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('add_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE asgard_fund_yggdrasil_events (
	tx	    		VARCHAR(64) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	vault_key		VARCHAR(90) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('asgard_fund_yggdrasil_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE bond_events (
	tx		    	VARCHAR(64) NOT NULL,
	chain			VARCHAR(8) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	memo			TEXT NOT NULL,
	bound_type		VARCHAR(32) NOT NULL,
	E8			    BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('bond_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE errata_events (
	in_tx			VARCHAR(64) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	rune_E8			BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('errata_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE fee_events (
	tx			VARCHAR(64) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	pool_deduct		BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('fee_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE gas_events (
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	rune_E8			BIGINT NOT NULL,
	tx_count		BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('gas_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE inactive_vault_events (
	add_asgard_addr		VARCHAR(90) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('inactive_vault_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE set_mimir_events (
	key			        VARCHAR(63) NOT NULL,
	value			    VARCHAR(127) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('set_mimir_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE message_events (
	from_addr		    VARCHAR(90) NOT NULL,
	action			    VARCHAR(31) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('message_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE new_node_events (
	node_addr		    VARCHAR(48) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('new_node_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE outbound_events (
	tx			    VARCHAR(64),
	chain			VARCHAR(8) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	memo			TEXT NOT NULL,
	in_tx			VARCHAR(64) NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('outbound_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE pool_events (
	asset			VARCHAR(60) NOT NULL,
	status			VARCHAR(64) NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('pool_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE refund_events (
	tx			    VARCHAR(64) NOT NULL,
	chain			VARCHAR(8) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	asset_2nd		VARCHAR(60),
	asset_2nd_E8	BIGINT NOT NULL,
	memo			TEXT,
	code			BIGINT NOT NULL,
	reason			TEXT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('refund_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE reserve_events (
	tx			    VARCHAR(64) NOT NULL,
	chain			VARCHAR(8) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	memo			TEXT NOT NULL,
	addr			VARCHAR(48) NOT NULL,
	E8			    BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('reserve_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE rewards_events (
	bond_E8			    BIGINT NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('rewards_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE rewards_event_entries (
	pool			    VARCHAR(60) NOT NULL,
	rune_E8			    BIGINT NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('rewards_event_entries', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE set_ip_address_events (
	node_addr		    VARCHAR(44) NOT NULL,
	ip_addr			    VARCHAR(45) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('set_ip_address_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE set_node_keys_events (
	node_addr   		VARCHAR(44) NOT NULL,
	secp256k1	    	VARCHAR(90) NOT NULL,
	ed25519			    VARCHAR(90) NOT NULL,
	validator_consensus	VARCHAR(90) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('set_node_keys_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE set_version_events (
	node_addr		    VARCHAR(44) NOT NULL,
	version			    VARCHAR(127) NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('set_version_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE slash_amounts (
	pool			    VARCHAR(60) NOT NULL,
	asset			    VARCHAR(60) NOT NULL,
	asset_E8		    BIGINT NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('slash_amounts', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE stake_events (
	pool			VARCHAR(60) NOT NULL,
	asset_tx		VARCHAR(64),
	asset_chain		VARCHAR(8),
	asset_addr		VARCHAR(90),
	asset_E8		BIGINT NOT NULL,
	stake_units		BIGINT NOT NULL,
	rune_tx			VARCHAR(64),
	rune_addr		VARCHAR(90),
	rune_E8			BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('stake_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE swap_events (
	tx			        VARCHAR(64) NOT NULL,
	chain			    VARCHAR(8) NOT NULL,
	from_addr		    VARCHAR(90) NOT NULL,
	to_addr			    VARCHAR(90) NOT NULL,
	from_asset		    VARCHAR(60) NOT NULL,
	from_E8			    BIGINT NOT NULL,
	to_E8			    BIGINT NOT NULL,
	memo			    TEXT NOT NULL,
	pool			    VARCHAR(60) NOT NULL,
	to_E8_min		    BIGINT NOT NULL,
	trade_slip_BP	    BIGINT NOT NULL,
	liq_fee_E8		    BIGINT NOT NULL,
	liq_fee_in_rune_E8	BIGINT NOT NULL,
	block_timestamp		BIGINT NOT NULL
);

SELECT create_hypertable('swap_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE transfer_events (
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	rune_E8			BIGINT NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('transfer_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE unstake_events (
	tx			    VARCHAR(64) NOT NULL,
	chain			VARCHAR(8) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	to_addr			VARCHAR(90) NOT NULL,
	asset			VARCHAR(60) NOT NULL,
	asset_E8		BIGINT NOT NULL,
	emit_asset_E8	BIGINT NOT NULL,
	emit_rune_E8	BIGINT NOT NULL,
	memo			TEXT NOT NULL,
	pool			VARCHAR(60) NOT NULL,
	stake_units		BIGINT NOT NULL,
	basis_points	BIGINT NOT NULL,
	asymmetry		DOUBLE PRECISION NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('unstake_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE update_node_account_status_events (
	node_addr		VARCHAR(90) NOT NULL,
	former			VARCHAR(31) NOT NULL,
	current			VARCHAR(31) NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('update_node_account_status_events', 'block_timestamp', chunk_time_interval => 86400000000000);


CREATE TABLE validator_request_leave_events (
	tx			    VARCHAR(64) NOT NULL,
	from_addr		VARCHAR(90) NOT NULL,
	node_addr		VARCHAR(90) NOT NULL,
	block_timestamp	BIGINT NOT NULL
);

SELECT create_hypertable('validator_request_leave_events', 'block_timestamp', chunk_time_interval => 86400000000000);
