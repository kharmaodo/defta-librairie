-- US-1714: optional metadata and crash recovery for experimental OCR.
ALTER TABLE cover_import_ocr_results ADD COLUMN policy_version TEXT;
ALTER TABLE cover_import_ocr_results ADD COLUMN psm INTEGER CHECK (psm IS NULL OR psm IN (6,11));
ALTER TABLE cover_import_ocr_results ADD COLUMN preprocessing TEXT CHECK (preprocessing IS NULL OR preprocessing IN ('original','grayscale-autocontrast'));
ALTER TABLE cover_import_jobs ADD COLUMN ocr_claim_token TEXT;
ALTER TABLE cover_import_jobs ADD COLUMN ocr_claim_until TEXT;
ALTER TABLE cover_import_jobs ADD COLUMN ocr_attempts INTEGER NOT NULL DEFAULT 0 CHECK (ocr_attempts >= 0);
