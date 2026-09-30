CREATE TABLE cover_import_review_suggestions (
 job_id TEXT NOT NULL REFERENCES cover_import_jobs(id),
 book_id INTEGER NOT NULL REFERENCES defta(id),
 origin TEXT NOT NULL CHECK(origin IN ('MANUAL','OCR')),
 rejected INTEGER NOT NULL DEFAULT 0 CHECK(rejected IN (0,1)),
 actor_user_id TEXT NOT NULL,
 updated_at TEXT NOT NULL,
 PRIMARY KEY(job_id,book_id)
);
