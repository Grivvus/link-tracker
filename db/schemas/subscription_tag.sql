CREATE TABLE "subscription_tag" (
    subscription_id BIGINT NOT NULL,
    tag_id BIGINT NOT NULL,
    PRIMARY KEY (subscription_id, tag_id),
    FOREIGN KEY (subscription_id) REFERENCES "subscription" (id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES "tag" (id) ON DELETE RESTRICT
);
