import { css } from '@emotion/css';
import { useState } from 'react';

import { DataFrame } from '@grafana/data';
import { Alert, Modal } from '@grafana/ui';
import { exportTraceAsMermaid } from 'app/features/inspector/utils/download';

import MermaidRenderer from './MermaidRenderer';

export const MermaidExportModal = ({
  isOpen,
  onDismiss,
  data,
  traceId,
}: {
  isOpen: boolean;
  onDismiss: () => void;
  data: DataFrame;
  traceId: string;
}) => {
  const [highlights, setHighlights] = useState<string[]>([]);
  (window as any).mermaidToggleSpan = (spanId: string) => {
    if (highlights.includes(spanId)) {
      setHighlights(highlights.filter((s) => s !== spanId));
    } else {
      setHighlights([...highlights, spanId]);
    }
  };
  const code = exportTraceAsMermaid(data, traceId, highlights);
  const handlers = data.fields[1].values.map((spanId) => `click ${spanId} call mermaidToggleSpan(${spanId})`);

  return (
    <Modal
      isOpen={isOpen}
      onDismiss={onDismiss}
      title={'Mermaid Export'}
      className={css`
        min-width: 60%;
      `}
    >
      <Alert severity="info" title="Click on spans to highlight them!" />
      <MermaidRenderer source={code + '\n' + handlers.join('\n')} id={'preview'} />
      <pre>{code}</pre>
    </Modal>
  );
};
