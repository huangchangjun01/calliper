# 一次性验证工具：查找 stocks 表中存在的大市值股票 + 统计 TSDB 日线数量。
from sqlalchemy import create_engine, text

main = create_engine('postgresql+psycopg2://calliper:10010hcj@localhost:5432/calliper_trading')
tsdb = create_engine('postgresql+psycopg2://calliper:10010hcj@localhost:5432/calliper_tsdb')

CANDIDATES = ['601318', '000651', '002415', '600030', '601166', '600016', '000333', '002594', '601888', '600276']

with main.connect() as mc:
    ph = ','.join(f":s{i}" for i in range(len(CANDIDATES)))
    params = {f"s{i}": s for i, s in enumerate(CANDIDATES)}
    rows = mc.execute(text(f"SELECT symbol, name FROM stocks WHERE symbol IN ({ph}) ORDER BY symbol"), params).fetchall()
    print('candidates in stocks:')
    for r in rows:
        print(' ', r[0], r[1])

with tsdb.connect() as tc:
    total = tc.execute(text("SELECT count(*) FROM stock_prices_daily")).scalar()
    print('tsdb daily total:', total)