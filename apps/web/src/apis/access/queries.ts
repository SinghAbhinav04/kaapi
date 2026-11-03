import { queryOptions } from '@tanstack/react-query';

import { accessApi } from './access.api';
import { accessKeys } from './const';

export const accessQueries = {
  overview: () => queryOptions({ queryKey: accessKeys.overview(), queryFn: ({ signal }) => accessApi.overview(signal) }),
  members: () => queryOptions({ queryKey: accessKeys.members(), queryFn: ({ signal }) => accessApi.members(signal) }),
};
