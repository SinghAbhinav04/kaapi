import { queryOptions } from '@tanstack/react-query';

import { offersKeys } from './const';
import { offersApi } from './offers.api';

export const offersQueries = {
  list: () => queryOptions({ queryKey: offersKeys.list(), queryFn: ({ signal }) => offersApi.list(signal) }),
};
