-- C:\Users\wasab\OneDrive\デスクトップ\WASABI\schema.sql

-- 得意先マスターテーブル
CREATE TABLE IF NOT EXISTS client_master (
  client_code TEXT PRIMARY KEY,
  client_name TEXT NOT NULL UNIQUE
);

-- 製品マスタ (WASABIの新しい仕様)
CREATE TABLE IF NOT EXISTS product_master (
    product_code TEXT PRIMARY KEY,
    yj_code TEXT,
    product_name TEXT,
    origin TEXT,
    kana_name TEXT,
    maker_name TEXT,
    usage_classification TEXT,
    package_form TEXT,
    yj_unit_name TEXT,
    yj_pack_unit_qty REAL,
    flag_poison INTEGER,
    flag_deleterious INTEGER,
    flag_narcotic INTEGER,
    flag_psychotropic INTEGER,
    flag_stimulant INTEGER,
    flag_stimulant_raw INTEGER,
    jan_pack_inner_qty REAL,
    jan_unit_code INTEGER,
    jan_pack_unit_qty REAL,
    nhi_price REAL,
    purchase_price REAL,
    supplier_wholesale TEXT
);

CREATE TABLE IF NOT EXISTS transaction_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  transaction_date TEXT,
  client_code TEXT,
  receipt_number TEXT,
  line_number TEXT,
  flag INTEGER,
  jan_code TEXT,
  yj_code TEXT,
  product_name TEXT,
  kana_name TEXT,
  usage_classification TEXT,
  package_form TEXT,
  package_spec TEXT,
  maker_name TEXT,
  dat_quantity REAL,
  jan_pack_inner_qty REAL,
  jan_quantity REAL,
  jan_pack_unit_qty REAL,
  jan_unit_name TEXT,
  jan_unit_code TEXT,
  yj_quantity REAL,
  yj_pack_unit_qty REAL,
  yj_unit_name TEXT,
  unit_price REAL,
  purchase_price REAL,
  supplier_wholesale TEXT,
  subtotal REAL,
  tax_amount REAL,
  tax_rate REAL,
  expiry_date TEXT,
  lot_number TEXT,
  flag_poison INTEGER,
  flag_deleterious INTEGER,
  flag_narcotic INTEGER,
  flag_psychotropic INTEGER,
  flag_stimulant INTEGER,
  flag_stimulant_raw INTEGER,
  process_flag_ma TEXT
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_transactions_unique_slip
  ON transaction_records(transaction_date, client_code, receipt_number, line_number)
  WHERE receipt_number != '';

-- JCSHMSマスタ
CREATE TABLE IF NOT EXISTS jcshms (
  JC000 TEXT, JC001 TEXT, JC002 TEXT, JC003 TEXT, JC004 TEXT, JC005 TEXT, JC006 TEXT, JC007 TEXT, JC008 TEXT, JC009 TEXT,
  JC010 TEXT, JC011 TEXT, JC012 TEXT, JC013 TEXT, JC014 TEXT, JC015 TEXT, JC016 TEXT, JC017 TEXT, JC018 TEXT, JC019 TEXT,
  JC020 TEXT, JC021 TEXT, JC022 TEXT, JC023 TEXT, JC024 TEXT, JC025 TEXT, JC026 TEXT, JC027 TEXT, JC028 TEXT, JC029 TEXT,
  JC030 TEXT, JC031 TEXT, JC032 TEXT, JC033 TEXT, JC034 TEXT, JC035 TEXT, JC036 TEXT, JC037 TEXT, JC038 TEXT, JC039 TEXT,
  JC040 TEXT, JC041 TEXT, JC042 TEXT, JC043 TEXT, JC044 REAL, JC045 TEXT, JC046 TEXT, JC047 TEXT, JC048 TEXT, JC049 TEXT,
  JC050 REAL, JC051 TEXT, JC052 TEXT, JC053 TEXT, JC054 TEXT, JC055 TEXT, JC056 TEXT, JC057 TEXT, JC058 TEXT, JC059 TEXT,
  JC060 TEXT, JC061 INTEGER, JC062 INTEGER, JC063 INTEGER, JC064 INTEGER, JC065 INTEGER, JC066 INTEGER, JC067 TEXT, JC068 TEXT, JC069 TEXT,
  JC070 TEXT, JC071 TEXT, JC072 TEXT, JC073 TEXT, JC074 TEXT, JC075 TEXT, JC076 TEXT, JC077 TEXT, JC078 TEXT, JC079 TEXT,
  JC080 TEXT, JC081 TEXT, JC082 TEXT, JC083 TEXT, JC084 TEXT, JC085 TEXT, JC086 TEXT, JC087 TEXT, JC088 TEXT, JC089 TEXT,
  JC090 TEXT, JC091 TEXT, JC092 TEXT, JC093 TEXT, JC094 TEXT, JC095 TEXT, JC096 TEXT, JC097 TEXT, JC098 TEXT, JC099 TEXT,
  JC100 TEXT, JC101 TEXT, JC102 TEXT, JC103 TEXT, JC104 TEXT, JC105 TEXT, JC106 TEXT, JC107 TEXT, JC108 TEXT, JC109 TEXT,
  JC110 TEXT, JC111 TEXT, JC112 TEXT, JC113 TEXT, JC114 TEXT, JC115 TEXT, JC116 TEXT, JC117 TEXT, JC118 TEXT, JC119 TEXT,
  JC120 TEXT, JC121 TEXT, JC122 TEXT, JC123 TEXT, JC124 TEXT,
  PRIMARY KEY(JC000)
);

-- JANCODEマスタ
CREATE TABLE IF NOT EXISTS jancode (
  JA000 TEXT, JA001 TEXT, JA002 TEXT, JA003 TEXT, JA004 TEXT, JA005 TEXT, JA006 REAL, JA007 TEXT, JA008 REAL, JA009 TEXT,
  JA010 TEXT, JA011 TEXT, JA012 TEXT, JA013 TEXT, JA014 TEXT, JA015 TEXT, JA016 TEXT, JA017 TEXT, JA018 TEXT, JA019 TEXT,
  JA020 TEXT, JA021 TEXT, JA022 TEXT, JA023 TEXT, JA024 TEXT, JA025 TEXT, JA026 TEXT, JA027 TEXT, JA028 TEXT, JA029 TEXT,
  PRIMARY KEY(JA001)
);

-- 自動採番シーケンス
CREATE TABLE IF NOT EXISTS code_sequences (
  name TEXT PRIMARY KEY,
  last_no INTEGER NOT NULL
);
INSERT OR IGNORE INTO code_sequences(name, last_no) VALUES ('MA2Y', 0);
INSERT OR IGNORE INTO code_sequences(name, last_no) VALUES ('CL', 0);

-- 検索を高速化するためのインデックスを追加
CREATE INDEX IF NOT EXISTS idx_transactions_jan_code ON transaction_records (jan_code);
CREATE INDEX IF NOT EXISTS idx_transactions_date ON transaction_records (transaction_date);
CREATE INDEX IF NOT EXISTS idx_transactions_flag ON transaction_records (flag);
CREATE INDEX IF NOT EXISTS idx_product_master_kana_name ON product_master (kana_name);

-- デッドストックリストテーブル
CREATE TABLE IF NOT EXISTS dead_stock_list (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  product_code TEXT NOT NULL,
  yj_code TEXT,
  package_form TEXT,
  jan_pack_inner_qty REAL,
  yj_unit_name TEXT,
  stock_quantity_jan REAL NOT NULL,
  expiry_date TEXT,
  lot_number TEXT,
  created_at TEXT NOT NULL,
  UNIQUE(product_code, expiry_date, lot_number)
);

CREATE INDEX IF NOT EXISTS idx_tx_jan_date
  ON transaction_records(jan_code, transaction_date);

-- 新しい予製レコードテーブル (transaction_recordsを模倣)
CREATE TABLE IF NOT EXISTS precomp_records (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  transaction_date TEXT,
  client_code TEXT, -- 患者番号を格納
  receipt_number TEXT,
  line_number TEXT,
  jan_code TEXT,
  yj_code TEXT,
  product_name TEXT,
  kana_name TEXT,
  usage_classification TEXT,
  package_form TEXT,
  package_spec TEXT,
  maker_name TEXT,
  jan_pack_inner_qty REAL,
  jan_quantity REAL,
  jan_pack_unit_qty REAL,
  jan_unit_name TEXT,
  jan_unit_code TEXT,
  yj_quantity REAL,
  yj_pack_unit_qty REAL,
  yj_unit_name TEXT,
  purchase_price REAL,
  supplier_wholesale TEXT,
  created_at TEXT NOT NULL,
  status TEXT NOT NULL DEFAULT 'active', -- status列をここに追加
  UNIQUE(client_code, jan_code)
);

-- 卸業者マスターテーブル
CREATE TABLE IF NOT EXISTS wholesalers (
  wholesaler_code TEXT PRIMARY KEY,
  wholesaler_name TEXT NOT NULL
);

-- 発注残管理テーブル (改善版)
CREATE TABLE IF NOT EXISTS backorders (
  -- ▼ 複合主キー ▼
  yj_code TEXT NOT NULL,
  package_form TEXT NOT NULL,
  jan_pack_inner_qty REAL NOT NULL,
  yj_unit_name TEXT NOT NULL,
  -- ▲ 複合主キーここまで ▲

  order_date TEXT NOT NULL,
  yj_quantity REAL NOT NULL,
  product_name TEXT, -- 表示用

  PRIMARY KEY (yj_code, package_form, jan_pack_inner_qty, yj_unit_name)
);

-- ▼▼▼ [追加] パフォーマンス改善のためのインデックス ▼▼▼
CREATE INDEX IF NOT EXISTS idx_transactions_receipt_number ON transaction_records (receipt_number);
CREATE INDEX IF NOT EXISTS idx_transactions_process_flag_ma ON transaction_records (process_flag_ma);
CREATE INDEX IF NOT EXISTS idx_transactions_flag_date ON transaction_records (flag, transaction_date);
-- ▲▲▲ [追加ここまで] ▲▲▲