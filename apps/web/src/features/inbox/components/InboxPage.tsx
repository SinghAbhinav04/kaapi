'use client';

import { useState } from 'react';

import { ComposeDialog, MailActions } from '@/features/operations';
import { Card } from '@/features/staff-shell';
import { cx } from '@/shared/lib/cx';

import { useInboxView } from '../hooks';
import { EnquiryDetail } from './EnquiryDetail';
import { EnquiryList } from './EnquiryList';
import styles from './Inbox.module.css';
import { InboundDialog } from './InboundDialog';

export function InboxPage() {
  const { lane, open, search, setSearch, inbox, all, list, navigate } = useInboxView();
  const [pasting, setPasting] = useState(false);
  const [composing, setComposing] = useState(false);

  return (
    <>
      <MailActions onCompose={() => setComposing(true)} />
      <Card className={cx(styles.inbox, open && styles.hasOpen)}>
        <EnquiryList
          lane={lane}
          list={list}
          all={all}
          open={open}
          search={search}
          onSearch={setSearch}
          onOpen={(ref) => navigate({ open: ref })}
          onLane={(l) => navigate({ lane: l, open: null })}
          loading={inbox.isPending}
          stale={inbox.isPlaceholderData}
          error={inbox.isError && !inbox.data ? { message: inbox.error.message, retry: () => inbox.refetch() } : null}
          onPaste={() => setPasting(true)}
        />
        <EnquiryDetail enquiryRef={open} onClose={() => navigate({ open: null })} />
        <InboundDialog
          open={pasting}
          onClose={() => setPasting(false)}
          onCreated={(ref) => {
            setPasting(false);
            navigate({ open: ref });
          }}
        />
      </Card>
      <ComposeDialog open={composing} onClose={() => setComposing(false)} />
    </>
  );
}
