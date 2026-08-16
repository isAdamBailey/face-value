CREATE TABLE searches (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_email      TEXT        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'pending',
                    -- pending | identifying | pricing | complete | failed
    error_message   TEXT,

    image_key       TEXT        NOT NULL,   -- opaque key for ImageStore
    image_width     INT,
    image_height    INT,

    -- vision output
    title           TEXT,                   -- "Sony TC-377 reel-to-reel tape deck"
    brand           TEXT,
    model           TEXT,
    category        TEXT,
    condition_notes TEXT,
    search_query    TEXT,                   -- the string actually sent to eBay
    vision_model    TEXT,                   -- e.g. "Qwen/Qwen2.5-VL-72B-Instruct"
    vision_raw      JSONB,                  -- full parsed model response
    confidence      NUMERIC(3,2),           -- 0.00-1.00, model self-reported

    -- pricing rollup
    price_source    TEXT,                   -- 'ebay_browse' | 'ebay_sold' | ...
    currency        TEXT,
    comp_count      INT,
    price_mean      NUMERIC(12,2),
    price_median    NUMERIC(12,2),
    price_min       NUMERIC(12,2),
    price_max       NUMERIC(12,2),
    price_trimmed_mean NUMERIC(12,2),       -- the headline number

    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX searches_user_created_idx ON searches (user_email, created_at DESC);

CREATE TABLE comps (
    id            BIGSERIAL PRIMARY KEY,
    search_id     UUID NOT NULL REFERENCES searches(id) ON DELETE CASCADE,
    external_id   TEXT NOT NULL,            -- eBay itemId
    title         TEXT NOT NULL,
    price         NUMERIC(12,2) NOT NULL,
    currency      TEXT NOT NULL,
    condition     TEXT,
    buying_option TEXT,                     -- FIXED_PRICE | AUCTION | BEST_OFFER
    item_url      TEXT,
    thumbnail_url TEXT,
    seller_country TEXT,
    excluded      BOOLEAN NOT NULL DEFAULT false,  -- outlier-trimmed
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX comps_search_idx ON comps (search_id);
