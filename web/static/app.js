(function () {
  'use strict';

  function fetchJSON(url, opts) {
    return fetch(url, opts).then(function (resp) {
      if (!resp.ok) throw new Error('HTTP ' + resp.status);
      return resp.json();
    });
  }

  function fmtDate(d) {
    var y = d.getFullYear();
    var m = String(d.getMonth() + 1).padStart(2, '0');
    var day = String(d.getDate()).padStart(2, '0');
    return y + '-' + m + '-' + day;
  }

  function escapeHtml(s) {
    if (!s) return '';
    var div = document.createElement('div');
    div.textContent = s;
    return div.innerHTML;
  }

  function statusBadge(status) {
    var cls = 'badge-queued';
    if (status === 'ok' || status === 'done' || status === 'completed') cls = 'badge-success';
    else if (status === 'running' || status === 'active') cls = 'badge-running';
    else if (status === 'failed' || status === 'error') cls = 'badge-failed';
    return '<span class="badge ' + cls + '">' + escapeHtml(status) + '</span>';
  }

  /* ===== Quick date buttons ===== */
  var quickBtns = document.querySelectorAll('.quick-dates button');
  quickBtns.forEach(function (btn) {
    btn.addEventListener('click', function () {
      var now = new Date();
      var startInput = document.querySelector('input[name="start"]');
      var endInput = document.querySelector('input[name="end"]');
      var range = btn.dataset.range;
      var start, end;
      if (range === 'today') { start = now; end = now; }
      else if (range === 'yesterday') { start = new Date(now - 86400000); end = start; }
      else if (range === '7d') { end = now; start = new Date(now - 6 * 86400000); }
      else if (range === '30d') { end = now; start = new Date(now - 29 * 86400000); }
      startInput.value = fmtDate(start);
      endInput.value = fmtDate(end);
    });
  });

  /* ===== Query form ===== */
  var queryForm = document.getElementById('query-form');
  if (queryForm) {
    var currentPage = 1;

    function doQuery(page) {
      currentPage = page || 1;
      var params = new URLSearchParams(new FormData(queryForm));
      params.set('page', currentPage);

      var bar = document.getElementById('result-bar');
      var spinner = document.getElementById('result-spinner');
      var info = document.getElementById('result-info');
      bar.style.display = 'flex';
      spinner.style.display = 'inline-flex';
      info.style.display = 'none';

      fetchJSON('/api/logs?' + params.toString()).then(function (data) {
        spinner.style.display = 'none';
        info.style.display = 'inline';

        var total = data.total || 0;
        var pageSize = data.page_size || 100;
        var pages = Math.ceil(total / pageSize);
        info.innerHTML = '共 <strong>' + total + '</strong> 条' + (pages > 1 ? ' · 第 ' + currentPage + '/' + pages + ' 页' : '');

        var tbody = document.getElementById('result-tbody');
        tbody.innerHTML = '';
        if (!data.rows || data.rows.length === 0) {
          tbody.innerHTML = '<tr class="placeholder-row"><td colspan="11">无匹配记录</td></tr>';
          document.getElementById('pagination').innerHTML = '';
          return;
        }
        data.rows.forEach(function (row) {
          var tr = document.createElement('tr');
          tr.innerHTML =
            '<td>' + escapeHtml(row.ts) + '</td>' +
            '<td>' + escapeHtml(row.device_ip) + '</td>' +
            '<td>' + escapeHtml(row.src_ip) + '</td>' +
            '<td>' + row.src_port + '</td>' +
            '<td>' + escapeHtml(row.dst_ip) + '</td>' +
            '<td>' + (row.dst_country ? escapeHtml(row.dst_country) : '<span class="muted">—</span>') + '</td>' +
            '<td>' + row.dst_port + '</td>' +
            '<td>' + row.protocol + '</td>' +
            '<td>' + escapeHtml(row.translated_ip) + '</td>' +
            '<td>' + row.translated_port + '</td>' +
            '<td class="muted">' + escapeHtml(row.source_file) + '</td>';
          tbody.appendChild(tr);
        });

        renderPagination(pages, currentPage);
      }).catch(function (err) {
        spinner.style.display = 'none';
        info.style.display = 'inline';
        info.textContent = '查询失败: ' + err.message;
      });
    }

    function renderPagination(pages, current) {
      var pag = document.getElementById('pagination');
      pag.innerHTML = '';
      if (pages <= 1) return;

      function addBtn(label, page, opts) {
        var btn = document.createElement('button');
        btn.textContent = label;
        if (page === current) btn.classList.add('active');
        if (opts && opts.disabled) btn.disabled = true;
        btn.onclick = function () { doQuery(page); };
        pag.appendChild(btn);
      }

      addBtn('‹', current - 1, { disabled: current <= 1 });

      var start = Math.max(1, current - 4);
      var end = Math.min(pages, current + 4);
      for (var i = start; i <= end; i++) addBtn(i, i);

      addBtn('›', current + 1, { disabled: current >= pages });
    }

    queryForm.addEventListener('submit', function (e) {
      e.preventDefault();
      doQuery(1);
    });

    var exportBtn = document.getElementById('export-btn');
    if (exportBtn) {
      exportBtn.addEventListener('click', function () {
        var params = new URLSearchParams(new FormData(queryForm));
        params.set('format', 'csv');
        fetchJSON('/api/exports?' + params.toString()).then(function (data) {
          alert('导出任务已创建，ID: ' + data.job_id + '\n可在任务页查看进度');
        }).catch(function (err) {
          alert('导出失败: ' + err.message);
        });
      });
    }
  }

  /* ===== Tasks page ===== */
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

  /* ===== IP Ranges page ===== */
  var rangeForm = document.getElementById('ip-range-form');
  if (rangeForm) {
    rangeForm.addEventListener('submit', function (e) {
      e.preventDefault();
      var params = new URLSearchParams(new FormData(rangeForm));
      fetch('/api/ip-ranges', { method: 'POST', body: params }).then(function (r) {
        if (!r.ok) throw new Error('HTTP ' + r.status);
        return r.json();
      }).then(function () {
        rangeForm.reset();
        loadRanges();
      }).catch(function (err) {
        alert('添加失败: ' + err.message);
      });
    });
    loadRanges();
  }

  function loadRanges() {
    fetchJSON('/api/ip-ranges').then(function (data) {
      var tbody = document.getElementById('ranges-tbody');
      if (!tbody) return;
      tbody.innerHTML = '';
      var ranges = data.ranges || [];
      if (ranges.length === 0) {
        tbody.innerHTML = '<tr class="placeholder-row"><td colspan="4">暂无自定义 IP 段</td></tr>';
        return;
      }
      ranges.forEach(function (rng) {
        var tr = document.createElement('tr');
        var delBtn = document.createElement('button');
        delBtn.className = 'btn btn-danger btn-sm';
        delBtn.textContent = '删除';
        delBtn.onclick = function () {
          if (!confirm('确认删除 ' + rng.cidr + ' ?')) return;
          fetch('/api/ip-ranges?cidr=' + encodeURIComponent(rng.cidr), { method: 'DELETE' })
            .then(function (r) { if (!r.ok) throw new Error('HTTP ' + r.status); return r.json(); })
            .then(function () { loadRanges(); })
            .catch(function (err) { alert('删除失败: ' + err.message); });
        };
        tr.innerHTML = '<td>' + escapeHtml(rng.cidr) + '</td><td>' + escapeHtml(rng.name) + '</td><td>' + escapeHtml(rng.country || '') + '</td><td></td>';
        tr.lastElementChild.appendChild(delBtn);
        tbody.appendChild(tr);
      });
    });
  }

  function loadJobs() {
    fetchJSON('/api/jobs').then(function (data) {
      var tbody = document.getElementById('jobs-tbody');
      if (!tbody) return;
      tbody.innerHTML = '';
      if (!data.jobs || data.jobs.length === 0) {
        tbody.innerHTML = '<tr class="placeholder-row"><td colspan="7">暂无任务</td></tr>';
        return;
      }
      data.jobs.forEach(function (job) {
        var tr = document.createElement('tr');
        tr.innerHTML =
          '<td>' + escapeHtml(job.id) + '</td>' +
          '<td>' + escapeHtml(job.type) + '</td>' +
          '<td>' + statusBadge(job.status) + '</td>' +
          '<td>' + job.progress + '/' + job.total + '</td>' +
          '<td class="muted">' + escapeHtml(job.source) + '</td>' +
          '<td>' + escapeHtml(job.created_at) + '</td>' +
          '<td>' + (job.error ? '<span style="color:var(--danger)">' + escapeHtml(job.error) + '</span>' : '<span class="muted">—</span>') + '</td>';
        tbody.appendChild(tr);
      });
    });
  }
})();
