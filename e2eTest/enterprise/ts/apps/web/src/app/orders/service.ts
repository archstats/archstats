import { log } from '@acme/shared';
import type { Invoice } from '../billing/invoice';
import { Button } from '@acme/ui/Button';
import { helper } from '@app/billing/invoice';
import lodash from 'lodash';
const legacy = require('@acme/shared');
export const make = (): Invoice => { log('x'); Button(); helper; lodash; legacy; return { id: '1' }; };
