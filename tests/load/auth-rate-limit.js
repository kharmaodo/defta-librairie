import http from 'k6/http';
import {check, sleep} from 'k6';
import {Counter, Rate} from 'k6/metrics';

const target = __ENV.AUTH_LOAD_BASE_URL;
if (__ENV.AUTH_LOAD_ALLOW !== 'isolated') {
  throw new Error('Refus : définir AUTH_LOAD_ALLOW=isolated pour exécuter ce test.');
}
if (!target) {
  throw new Error('Définir AUTH_LOAD_BASE_URL vers un environnement de test isolé.');
}

const limited = new Counter('auth_rate_limited');
const validAuthenticationResponse = new Rate('auth_expected_response');

export const options = {
  scenarios: {
    invalid_login_burst: {
      executor: 'per-vu-iterations',
      vus: Number(__ENV.AUTH_LOAD_VUS || 16),
      iterations: Number(__ENV.AUTH_LOAD_ITERATIONS || 2),
      maxDuration: '30s',
    },
  },
  thresholds: {
    auth_expected_response: ['rate==1'],
  },
};

function endpoint(path) {
  return target.replace(/\/$/, '') + path;
}

export default function () {
  const response = http.post(
    endpoint('/api/auth/login'),
    JSON.stringify({
      username: 'owasp-invalid-' + __VU + '-' + __ITER,
      password: 'invalid-password',
    }),
    {headers: {'Content-Type': 'application/json'}},
  );

  const expected = check(response, {
    'login invalid request is rejected': (r) => r.status === 401 || r.status === 429,
    'rejection does not disclose account state': (r) =>
      !r.body || !r.body.includes('account locked'),
  });
  validAuthenticationResponse.add(expected);

  if (response.status === 429) {
    limited.add(1);
    check(response, {
      'rate-limited response includes Retry-After': (r) => r.headers['Retry-After'] !== undefined,
      'rate-limited response has generic error': (r) => r.json('error') === 'rate_limited',
    });
  }

  sleep(0.1);
}
