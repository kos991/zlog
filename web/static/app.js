(function () {
  'use strict';

  function fetchJSON(url, opts) {
    return fetch(url, opts).then(function (resp) {
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      return resp.json();
    });
  }

  var queryForm = document.getElementById('query-form');
  if (queryForm) {
    queryForm.addEventListener('submit', function (e) {
      e.preventDefault();
      var params = new URLSearchParams(new FormData(queryForm));
      fetchJSON('/api/logs?' + params.toString()).then(function (data) {
        var tbody = document.getElementById('result-tbody');
        tbody.innerHTML = '';
        if (!data.rows || data.rows.length === 0) {
          tbody.innerHTML = '<tr><td colspan="12" class="placeholder">无结果</td></tr>';
          return;
        }
        data.rows.forEach(function (row) {
          var tr = document.createElement('tr');
          tr.innerHTML = '<td>' + row.ts + '</td><td>' + row.device_ip + '</td><td>' +
            row.log_type + '</td><td>' + row.src_ip + '</td><td>' + row.src_port + '</td><td>' +
            row.dst_ip + '</td><td>' + row.dst_country + '</td><td>' + row.dst_port + '</td><td>' +
            row.protocol + '</td><td>' + row.translated_ip + '</td><td>' + row.translated_port +
            '</td><td>' + row.source_file + '</td>';
          tbody.appendChild(tr);
        });
      }).catch(function (err) {
        alert('查询失败: ' + err.message);
      });
    });

    var exportBtn = document.getElementById('export-btn');
    if (exportBtn) {
      exportBtn.addEventListener('click', function () {
        var params = new URLSearchParams(new FormData(queryForm));
        params.set('format', 'csv');
        fetchJSON('/api/exports?' + params.toString()).then(function (data) {
          alert('导出任务已创建: ' + data.job_id);
        }).catch(function (err) {
          alert('导出失败: ' + err.message);
        });
      });
    }
  }

  var importBtn = document.getElementById('trigger-import');
  if (importBtn) {
    importBtn.addEventListener('click', function () {
      fetchJSON('/api/import', { method: 'POST' }).then(function () {
        alert('导入已启动');
        loadJobs();
      }).catch(function (err) {
        alert('导入失败: ' + err.message);
      });
    });
    loadJobs();
  }

  function loadJobs() {
    fetchJSON('/api/jobs').then(function (data) {
      var tbody = document.getElementById('jobs-tbody');
      if (!tbody) return;
      tbody.innerHTML = '';
      if (!data.jobs || data.jobs.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="placeholder">无任务</td></tr>';
        return;
      }
      data.jobs.forEach(function (job) {
        var tr = document.createElement('tr');
        tr.innerHTML = '<td>' + job.id + '</td><td>' + job.type + '</td><td>' +
          job.status + '</td><td>' + job.progress + '/' + job.total + '</td><td>' +
          job.source + '</td><td>' + job.created_at + '</td><td>' + (job.error || '') + '</td>';
        tbody.appendChild(tr);
      });
    });
  }
})();
