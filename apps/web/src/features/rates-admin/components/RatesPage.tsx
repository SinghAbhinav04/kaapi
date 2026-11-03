'use client';

import { hasPermission, useMe } from '@/apis/auth';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { addDays, formatDayMonth, formatWeekdayDay, todayIST } from '@/shared/lib/dates';
import { ErrorState, Segmented, Skeleton } from '@/shared/ui';

import { MAX_WEEKS_AHEAD, PLAN_OPTIONS, PRICE_PLAN, WINDOW_DAYS } from '../const';
import { useRatesView } from '../hooks';
import { gridKpis, weekStart } from '../lib';
import { RateEditor } from './RateEditor';
import { RatesGrid } from './RatesGrid';
import styles from './Rates.module.css';

export function RatesPage() {
  const view = useRatesView();
  const { data: me } = useMe();
  const kpi = gridKpis(view.grid.data);
  const thisWeek = weekStart(todayIST());
  const lastWeek = addDays(thisWeek, MAX_WEEKS_AHEAD * 7);
  const pct = kpi.allotment ? Math.round((kpi.sold / kpi.allotment) * 100) : 0;

  return (
    <div className={styles.page}>
      <StaffPageHeader
        title="Rates & allotment"
        description="What the website can sell, night by night. Agencies keep the rest of the 76 keys."
        actions={
          <>
            <Segmented label="Rate plan" options={PLAN_OPTIONS} value={view.plan} onChange={view.setPlan} />
            <div className={styles.window} role="group" aria-label="Dates">
              <button type="button" aria-label="Previous two weeks" disabled={view.from <= thisWeek} onClick={() => view.page(-2)}>
                ‹
              </button>
              <span>
                {formatDayMonth(view.from)} – {formatDayMonth(addDays(view.from, WINDOW_DAYS - 1))}
              </span>
              <button type="button" aria-label="Next two weeks" disabled={view.from >= lastWeek} onClick={() => view.page(2)}>
                ›
              </button>
            </div>
          </>
        }
      />

      <div className={styles.kpis}>
        <Card className={styles.kpi}>
          <span className={styles.kpiLabel}>Website nights sold</span>
          <p className={styles.kpiValue}>
            {kpi.sold} <small>of {kpi.allotment}</small>
          </p>
          <span className={styles.kpiBar}>
            <span style={{ width: `${pct}%` }} />
          </span>
        </Card>
        <Card className={styles.kpi}>
          <span className={styles.kpiLabel}>Sold out online</span>
          <p className={styles.kpiValue} data-alert={kpi.soldOut.length > 0 || undefined}>
            {kpi.soldOut.length} {kpi.soldOut.length === 1 ? 'night' : 'nights'}
          </p>
          <span className={styles.kpiSub}>{kpi.soldOut[0] ? `${kpi.soldOut[0].room}, ${formatWeekdayDay(kpi.soldOut[0].date)}${kpi.soldOut.length > 1 ? ' and more' : ''}` : 'Every room type has space'}</span>
        </Card>
        <Card className={styles.kpi}>
          <span className={styles.kpiLabel}>On hold right now</span>
          <p className={styles.kpiValue}>
            {kpi.held} {kpi.held === 1 ? 'room-night' : 'room-nights'}
          </p>
          <span className={styles.kpiSub}>Guests at checkout; holds end after 10 minutes</span>
        </Card>
        <Card dark className={`${styles.kpi} ${styles.kpiDark}`}>
          <span className={styles.kpiLabel}>Pricing rules</span>
          <p className={styles.kpiValue}>Weekends +15%</p>
          <span className={styles.kpiSub}>Festival dates carry their own uplift and minimum stay</span>
        </Card>
      </div>

      <Card className={styles.gridCard} aria-label="Nightly price and rooms left">
        {view.grid.isPending ? (
          <Skeleton height={420} radius={12} />
        ) : view.grid.isError && !view.grid.data ? (
          <ErrorState message={view.grid.error.message} onRetry={() => view.grid.refetch()} retrying={view.grid.isFetching} />
        ) : (
          view.grid.data && <RatesGrid grid={view.grid.data} selection={view.selection} onPick={view.pick} stale={view.grid.isPlaceholderData} />
        )}
        <div className={styles.legend}>
          <span className={styles.key}>
            <span className={`${styles.swatch} ${styles.free}`} />
            All rooms free
          </span>
          <span className={styles.key}>
            <span className={`${styles.swatch} ${styles.some}`} />
            Some sold
          </span>
          <span className={styles.key}>
            <span className={`${styles.swatch} ${styles.half}`} />
            Half or more sold
          </span>
          <span className={styles.key}>
            <span className={`${styles.swatch} ${styles.full}`} />
            Sold out online
          </span>
          <span className={styles.legendNote}>Price per night before GST · click nights in one row to edit</span>
        </div>
      </Card>

      {view.selection && view.row && view.nights.length > 0 && (
        <Card dark>
          <RateEditor
            key={`${view.selection.roomTypeId}-${view.selection.first}-${view.selection.last}-${view.plan}`}
            row={view.row}
            nights={view.nights}
            selection={view.selection}
            canPrice={hasPermission(me ?? null, 'rates:write')}
            canInventory={hasPermission(me ?? null, 'inventory:write')}
            pricePlanShown={view.plan === PRICE_PLAN}
            onCancel={view.clear}
            onSave={(body) => view.update.mutate(body)}
            saving={view.update.isPending}
            error={view.update.error}
            saved={view.update.data}
          />
        </Card>
      )}
    </div>
  );
}
