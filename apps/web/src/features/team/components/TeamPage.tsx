'use client';

import { useState } from 'react';

import { Card, StaffPageHeader } from '@/features/staff-shell';
import { Segmented } from '@/shared/ui';

import { COPY, PANELS, type Panel } from '../const';
import { PeoplePanel } from './PeoplePanel';
import { RolesPanel } from './RolesPanel';
import styles from './Team.module.css';

export function TeamPage() {
  const [panel, setPanel] = useState<Panel>('people');
  return (
    <div className={styles.page}>
      <StaffPageHeader title={COPY.title} description={COPY.description} actions={<Segmented label="Show" options={PANELS} value={panel} onChange={setPanel} />} />
      <Card className={styles.card}>{panel === 'people' ? <PeoplePanel /> : <RolesPanel />}</Card>
    </div>
  );
}
