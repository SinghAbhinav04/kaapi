'use client';

import { useState } from 'react';

import { useAccessOverview, useCreateRole, useDeleteRole, useUpdateRole, type AccessRole, type PermissionInfo } from '@/apis/access';
import { plural } from '@/shared/lib/text';
import { Button, Checkbox, ErrorState, Pill, SelectField, Skeleton, TextField } from '@/shared/ui';

import { COPY, TEAM_LABEL, TEAM_OPTIONS } from '../const';
import { groupBy } from '../lib';
import styles from './Team.module.css';

const NEW = 'new';

export function RolesPanel() {
  const overview = useAccessOverview();
  const [selected, setSelected] = useState<string | null>(null);

  if (overview.isPending) return <Skeleton height={360} radius={12} />;
  if (overview.isError) return <ErrorState message={overview.error.message} onRetry={() => overview.refetch()} retrying={overview.isFetching} />;

  const roles = overview.data.roles.filter((r) => r.key !== 'GUEST');
  const current = selected ?? roles[0]?.id ?? NEW;
  const role = roles.find((r) => r.id === current);

  return (
    <div className={styles.roles}>
      <div>
        <ul className={styles.roleList}>
          {roles.map((r) => (
            <li key={r.id}>
              <button type="button" className={styles.roleItem} aria-current={r.id === current} onClick={() => setSelected(r.id)}>
                <span>
                  {r.name}
                  <span className={styles.sub}>{TEAM_LABEL[r.team]}</span>
                </span>
                <span className={styles.muted}>{plural(r.members, 'person', 'people')}</span>
              </button>
            </li>
          ))}
        </ul>
        <Button variant="quiet" onClick={() => setSelected(NEW)}>
          New role
        </Button>
      </div>
      <RoleEditor
        key={current}
        role={current === NEW ? undefined : role}
        catalogue={overview.data.permissions}
        onSaved={(id) => setSelected(id)}
        onDeleted={() => setSelected(null)}
      />
    </div>
  );
}

function RoleEditor({
  role,
  catalogue,
  onSaved,
  onDeleted,
}: {
  role?: AccessRole;
  catalogue: PermissionInfo[];
  onSaved: (id: string) => void;
  onDeleted: () => void;
}) {
  const [name, setName] = useState(role?.name ?? '');
  const [description, setDescription] = useState(role?.description ?? '');
  const [team, setTeam] = useState<string>(role?.team && role.team !== 'ADMIN' ? role.team : 'RESERVATIONS');
  const [perms, setPerms] = useState<string[]>(role?.permissions ?? ['enquiries:read']);
  const create = useCreateRole();
  const update = useUpdateRole();
  const remove = useDeleteRole();
  const locked = role?.system ?? false;
  const error = create.error ?? update.error ?? remove.error;

  const toggle = (key: string, on: boolean) => setPerms((p) => (on ? [...p, key] : p.filter((k) => k !== key)));
  const save = () => {
    const body = { name, description, team: team as 'RESERVATIONS' | 'SALES' | 'GM', permissions: perms };
    if (role) update.mutate({ id: role.id, body }, { onSuccess: (r) => onSaved(r.role.id) });
    else create.mutate(body, { onSuccess: (r) => onSaved(r.role.id) });
  };

  return (
    <div className={styles.editor}>
      <div className={styles.toolbar}>
        <h2 className={styles.dialogTitle}>{role ? role.name : 'New role'}</h2>
        {locked && <Pill tone="neutral">Built in</Pill>}
      </div>
      {locked && <p className={styles.muted}>{COPY.systemRole}</p>}
      <TextField label="Name" value={name} onChange={(e) => setName(e.target.value)} disabled={locked} error={error?.fieldErrors.name} />
      <TextField label="Description" optional value={description} onChange={(e) => setDescription(e.target.value)} disabled={locked} />
      <SelectField label="Team" value={team} onChange={(e) => setTeam(e.target.value)} options={TEAM_OPTIONS} hint={COPY.teamHint} disabled={locked} />
      <div className={styles.groups}>
        {groupBy(catalogue, (p) => p.group).map(([group, items]) => (
          <fieldset key={group} className={styles.group}>
            <legend>{group}</legend>
            {items.map((p) => (
              <Checkbox key={p.key} checked={perms.includes(p.key)} disabled={locked} onChange={(e) => toggle(p.key, e.target.checked)}>
                {p.label}
                <span className={styles.permDesc}>{p.description}</span>
              </Checkbox>
            ))}
          </fieldset>
        ))}
      </div>
      {error && (
        <p className={styles.error} role="alert">
          {Object.values(error.fieldErrors)[0] ?? error.message}
        </p>
      )}
      {!locked && (
        <div className={styles.row}>
          {role && role.members === 0 && (
            <Button variant="quiet" loading={remove.isPending} onClick={() => remove.mutate(role.id, { onSuccess: onDeleted })}>
              Delete role
            </Button>
          )}
          <Button onClick={save} loading={create.isPending || update.isPending} disabled={name.trim().length < 2}>
            {role ? 'Save changes' : 'Create role'}
          </Button>
        </div>
      )}
    </div>
  );
}
