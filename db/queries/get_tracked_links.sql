-- name: GetTrackedLinksCntGithub :one
select COUNT(*) from "link" l
where l.id in
    (select distinct link_id
    from "subscription" s)
AND l.url LIKE '%github%'
;

-- name: GetTrackedLinksCntStackoverflow :one
select COUNT(*) from "link" l
where l.id in
    (select distinct link_id
    from "subscription" s)
AND l.url LIKE '%stackoverflow%'
;
