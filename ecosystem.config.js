module.exports = {
  apps: [
    {
      name: "mirabellier-go",
      script: "./server",
      cwd: "/srv/mirabellier-backend-go",
      env: {
        PORT: "5000",
        DB_FILE: "./database.sqlite3",
        IMAGES_DIR: "./images",
        GIN_MODE: "release",
      },
      // Restart if crash
      autorestart: true,
      max_restarts: 10,
      restart_delay: 2000,
      // Watch for file changes (disable in prod, enable for dev)
      watch: false,
      // Logging
      out_file: "./logs/out.log",
      error_file: "./logs/err.log",
      log_date_format: "YYYY-MM-DD HH:mm:ss",
      merge_logs: true,
      // Graceful shutdown
      kill_timeout: 5000,
      listen_timeout: 3000,
    },
  ],
};
