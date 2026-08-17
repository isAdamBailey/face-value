// Not used by the Forge deploy — Forge's zero-downtime deployments generate
// their own PM2 config for Nuxt/Next sites (a site-<id>.json file with its
// own process name and port), invoked from the site's Deploy Script after
// $ACTIVATE_RELEASE(). See docs/DEPLOY.md. This file is only for running a
// production build under PM2 manually, outside of Forge.
module.exports = {
  apps: [
    {
      name: 'facevalue-web',
      script: '.output/server/index.mjs',
      cwd: __dirname,
      instances: 1,
      exec_mode: 'fork',
      env: {
        NODE_ENV: 'production',
        HOST: '127.0.0.1',
      },
    },
  ],
}
