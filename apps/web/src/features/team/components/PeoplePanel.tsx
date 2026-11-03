'use client';

import { useState } from 'react';

import { useAccessOverview, useMembers, useUpdateMember, type Member } from '@/apis/access';
import { useMe } from '@/apis/auth';
import { cx } from '@/shared/lib/cx';
import { formatTimestamp } from '@/shared/lib/dates';
import { Button, EmptyState, ErrorState, Pill, Skeleton } from '@/shared/ui';

import { COPY, TEAM_LABEL } from '../const';
import { MemberDialog } from './MemberDialog';
import { PasswordDialog } from './PasswordDialog';
import styles from './Team.module.css';

export function PeoplePanel() {
  const members = useMembers();
  const roles = useAccessOverview().data?.roles.filter((r) => r.key !== 'GUEST') ?? [];
  const update = useUpdateMember();
  const me = useMe().data;
  const [adding, setAdding] = useState(false);
  const [resetting, setResetting] = useState<Member | null>(null);

  if (members.isPending) return <Skeleton height={320} radius={12} />;
  if (members.isError) return <ErrorState message={members.error.message} onRetry={() => members.refetch()} retrying={members.isFetching} />;

  return (
    <>
      <div className={styles.toolbar}>
        <p className={styles.muted}>{COPY.signOutNote}</p>
        <Button variant="ink" onClick={() => setAdding(true)} disabled={roles.length === 0}>
          Add person
        </Button>
      </div>
      {update.isError && (
        <p className={styles.error} role="alert">
          {update.error.message}
        </p>
      )}
      {members.data.members.length === 0 ? (
        <EmptyState compact title="No staff yet">
          Add the first person to give them access.
        </EmptyState>
      ) : (
        <div className={styles.scroller}>
          <table className={styles.table}>
            <thead>
              <tr>
                <th scope="col">Person</th>
                <th scope="col">Role</th>
                <th scope="col">Team</th>
                <th scope="col">Status</th>
                <th scope="col">
                  <span className="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              {members.data.members.map((m) => {
                const self = m.id === me?.id;
                return (
                  <tr key={m.id} className={cx(!m.active && styles.inactive)}>
                    <td>
                      <strong>{m.name}</strong>
                      {self && <span className={styles.sub}>You</span>}
                      <span className={styles.sub}>{m.email}</span>
                    </td>
                    <td>
                      <label>
                        <span className="sr-only">Role for {m.name}</span>
                        <select
                          className={styles.roleSelect}
                          value={m.role_id}
                          disabled={self || update.isPending}
                          onChange={(e) => update.mutate({ id: m.id, body: { role_id: e.target.value } })}
                        >
                          {roles.map((r) => (
                            <option key={r.id} value={r.id}>
                              {r.name}
                            </option>
                          ))}
                        </select>
                      </label>
                    </td>
                    <td>{TEAM_LABEL[m.team] ?? m.team}</td>
                    <td>
                      <Pill tone={m.active ? 'estate' : 'neutral'}>{m.active ? 'Active' : 'Deactivated'}</Pill>
                      <span className={styles.sub}>Added {formatTimestamp(m.created_at)}</span>
                    </td>
                    <td>
                      <div className={styles.actions}>
                        <Button variant="quiet" onClick={() => setResetting(m)}>
                          New password
                        </Button>
                        {!self && (
                          <Button variant="quiet" onClick={() => update.mutate({ id: m.id, body: { active: !m.active } })}>
                            {m.active ? 'Deactivate' : 'Reactivate'}
                          </Button>
                        )}
                      </div>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
      <MemberDialog open={adding} roles={roles} onClose={() => setAdding(false)} />
      <PasswordDialog member={resetting} onClose={() => setResetting(null)} />
    </>
  );
}
