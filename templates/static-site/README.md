# static-site template

Skeleton tree for the pattern documented in `docs/patterns/static-site-deploy.md`. Copy into the app repo and adapt.

```sh
cp -r clusterkit/templates/static-site/web        myapp/web
cp -r clusterkit/templates/static-site/charts/SITE myapp/charts/<site-name>
cp clusterkit/templates/static-site/scripts/deploy-web.sh myapp/scripts/deploy-web.sh
chmod +x myapp/scripts/deploy-web.sh
```

Replace `SITE`, `<APP>`, `<DOMAIN>` placeholders. See the pattern doc for the role split and gotchas before deploying.
