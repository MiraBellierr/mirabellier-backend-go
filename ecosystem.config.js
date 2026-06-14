module.exports = {
  apps: [
    {
      name: "mirabellier-go",
      script: "./server",
      cwd: "/srv/mirabellier-backend-go",
      env_file: ".env",
      autorestart: true,
      max_restarts: 10,
      restart_delay: 2000,
      watch: false,
      out_file: "./logs/out.log",
      error_file: "./logs/err.log",
      log_date_format: "YYYY-MM-DD HH:mm:ss",
      merge_logs: true,
      kill_timeout: 5000,
      listen_timeout: 3000,
      // Only use PM2's internal env vars, not the system's
      // PM2_PUBLIC_KEY, etc.
    },
  ],
};
