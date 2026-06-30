#!/usr/bin/env bash
set -euo pipefail

LOG_SEARCH_BIN="${LOG_SEARCH_BIN:-log-search}"
LOG_DIR="${LOG_SEARCH_LOG_DIR:-/data/sangfor_fw_log}"
EXPORT_PATH="${LOG_SEARCH_EXPORT:-/data/query_result.log}"

echo -e "\033[32m=============================================\033[0m"
echo -e "\033[32m   深信服防火墙日志查询工具（Go + zvec 版）\033[0m"
echo -e "\033[32m   支持：导入数据库、IP 查询、时间段查询、组合查询、结果导出\033[0m"
echo -e "\033[32m=============================================\033[0m"
echo ""

if ! command -v "${LOG_SEARCH_BIN}" >/dev/null 2>&1; then
    echo -e "\033[31m错误：未找到 ${LOG_SEARCH_BIN}，请先安装二进制包或设置 LOG_SEARCH_BIN\033[0m"
    exit 1
fi

echo "请选择操作："
echo "1. 导入日志到 zvec 数据库"
echo "2. 按 IP 查询"
echo "3. 按时间段查询"
echo "4. IP + 时间段组合查询"
echo "5. 指定日志文件片段查询"
read -r -p "请输入（1/2/3/4/5）：" MODE

case "${MODE}" in
    1)
        "${LOG_SEARCH_BIN}" import --log-dir "${LOG_DIR}"
        ;;
    2)
        read -r -p "请输入要查询的 IP：" QUERY_IP
        "${LOG_SEARCH_BIN}" query --ip "${QUERY_IP}"
        ;;
    3)
        read -r -p "开始时间（YYYY / YYYYMM / YYYYMMDD）：" START_DATE
        read -r -p "结束时间（YYYY / YYYYMM / YYYYMMDD）：" END_DATE
        "${LOG_SEARCH_BIN}" query --start "${START_DATE}" --end "${END_DATE}"
        ;;
    4)
        read -r -p "请输入要查询的 IP：" QUERY_IP
        read -r -p "开始时间（YYYY / YYYYMM / YYYYMMDD）：" START_DATE
        read -r -p "结束时间（YYYY / YYYYMM / YYYYMMDD）：" END_DATE
        "${LOG_SEARCH_BIN}" query --ip "${QUERY_IP}" --start "${START_DATE}" --end "${END_DATE}"
        ;;
    5)
        read -r -p "请输入文件名片段（如 20260429.gz）：" TARGET_FILE
        read -r -p "请输入 IP（直接回车则不按 IP 过滤）：" QUERY_IP
        if [ -n "${QUERY_IP}" ]; then
            "${LOG_SEARCH_BIN}" query --file "${TARGET_FILE}" --ip "${QUERY_IP}"
        else
            "${LOG_SEARCH_BIN}" query --file "${TARGET_FILE}"
        fi
        ;;
    *)
        echo -e "\033[31m输入错误\033[0m"
        exit 1
        ;;
esac

echo ""
read -r -p "是否导出同条件结果到 ${EXPORT_PATH}? (y/n) " EXPORT
if [[ "${EXPORT}" == "y" || "${EXPORT}" == "Y" ]]; then
    case "${MODE}" in
        2)
            "${LOG_SEARCH_BIN}" query --ip "${QUERY_IP}" --export "${EXPORT_PATH}" >/dev/null
            ;;
        3)
            "${LOG_SEARCH_BIN}" query --start "${START_DATE}" --end "${END_DATE}" --export "${EXPORT_PATH}" >/dev/null
            ;;
        4)
            "${LOG_SEARCH_BIN}" query --ip "${QUERY_IP}" --start "${START_DATE}" --end "${END_DATE}" --export "${EXPORT_PATH}" >/dev/null
            ;;
        5)
            if [ -n "${QUERY_IP:-}" ]; then
                "${LOG_SEARCH_BIN}" query --file "${TARGET_FILE}" --ip "${QUERY_IP}" --export "${EXPORT_PATH}" >/dev/null
            else
                "${LOG_SEARCH_BIN}" query --file "${TARGET_FILE}" --export "${EXPORT_PATH}" >/dev/null
            fi
            ;;
        *)
            echo "导入操作没有查询结果可导出"
            ;;
    esac
fi

echo -e "\033[32m操作结束\033[0m"
